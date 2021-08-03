package rest

import (
	"encoding/json"
	"goharvest2/cmd/poller/collector"
	"goharvest2/cmd/poller/plugin"
	"goharvest2/pkg/api/ontapi/rest"
	"goharvest2/pkg/errors"
	"goharvest2/pkg/matrix"
	//"goharvest2/pkg/tree/node"
	"strconv"
	"strings"
	"time"
)

type Rest struct {
	*collector.AbstractCollector
	client         *rest.Client
	apiPath        string
	instanceKeys   []string
	instanceLabels map[string]string
}

func init() {
	plugin.RegisterModule(Rest{})
}

func (Rest) HarvestModule() plugin.ModuleInfo {
	return plugin.ModuleInfo{
		ID:  "harvest.collector.rest",
		New: func() plugin.Module { return new(Rest) },
	}
}

func (r *Rest) Init(a *collector.AbstractCollector) error {

	var err error

	r.AbstractCollector = a
	if err = collector.Init(r); err != nil {
		return err
	}

	if r.client, err = r.getClient(); err != nil {
		return err
	}

	if err = r.client.Init(5); err != nil {
		return err
	}

	r.Logger.Info().Msgf("connected to %s: %s", r.client.ClusterName(), r.client.Info())

	r.Matrix.SetGlobalLabel("cluster", r.client.ClusterName())

	if err = r.initCache(r.getTemplateFn(), r.client.Version()); err != nil {
		return err
	}
	r.Logger.Info().Msgf("intialized cache with %d metrics", len(r.Matrix.GetMetrics()))
	return nil
}

func (r *Rest) getClient() (*rest.Client, error) {
	var (
		addr, x             string
		useInsecureTls      bool
		certAuth, basicAuth [2]string
	)

	if addr = r.Params.GetChildContentS("addr"); addr == "" {
		return nil, errors.New(errors.MISSING_PARAM, "addr")
	}

	if x = r.Params.GetChildContentS("use_insecure_tls"); x != "" {
		useInsecureTls, _ = strconv.ParseBool(x)
	}

	// set authentication method
	if r.Params.GetChildContentS("auth_style") == "certificate_auth" {

		certAuth[0] = r.Params.GetChildContentS("ssl_cert")
		certAuth[1] = r.Params.GetChildContentS("ssl_key")

		return rest.New(addr, &certAuth, nil, useInsecureTls)
	}

	basicAuth[0] = r.Params.GetChildContentS("username")
	basicAuth[1] = r.Params.GetChildContentS("password")

	return rest.New(addr, nil, &basicAuth, useInsecureTls)
}

func (r *Rest) getTemplateFn() string {
	var fn string
	if r.Params.GetChildS("objects") != nil {
		fn = r.Params.GetChildS("objects").GetChildContentS(r.Object)
	}
	return fn
}

func (r *Rest) PollData() (*matrix.Matrix, error) {

	var (
		content      []byte
		data         map[string]interface{}
		records      []interface{}
		ok           bool
		count        uint64
		apiD, parseD time.Duration
		startTime    time.Time
		err          error
	)

	r.Logger.Info().Msgf("starting data poll")
	r.Matrix.Reset()

	startTime = time.Now()
	if content, err = r.client.Get(r.apiPath, map[string]string{"fields": "*,"}); err != nil {
		return nil, err
	}
	apiD = time.Since(startTime)

	startTime = time.Now()
	if err = json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	parseD = time.Since(startTime)

	if records, ok = data["records"].([]interface{}); !ok {
		return nil, errors.New(errors.ERR_NO_INSTANCE, "no "+r.Object+" instances on cluster")
	}

	//r.Logger.Debug().Msgf("raw data:\n%s", string(content))
	r.Logger.Debug().Msgf("extracted %d [%s] instances", len(records), r.Object)

	for _, i := range records {

		var (
			instanceData map[string]interface{}
			instanceKey  string
			instance     *matrix.Instance
		)

		if instanceData, ok = i.(map[string]interface{}); !ok {
			r.Logger.Warn().Msg("skip instance")
			continue
		}

		// extract instance key(s)
		for _, k := range r.instanceKeys {
			if value, has := extractLabel(instanceData, k); has {
				instanceKey += value
			} else {
				r.Logger.Warn().Msgf("skip instance, missing key [%s]", k)
				break
			}
		}

		if instanceKey == "" {
			continue
		}

		if instance = r.Matrix.GetInstance(instanceKey); instance == nil {
			if instance, err = r.Matrix.NewInstance(instanceKey); err != nil {
				r.Logger.Error().Msgf("NewInstance [key=%s]: %v", instanceKey, err)
				continue
			}
		}

		for label, display := range r.instanceLabels {
			if value, has := extractLabel(instanceData, label); has {
				instance.SetLabel(display, value)
				count++
			}
		}

		for key, metric := range r.Matrix.GetMetrics() {

			if metric.GetProperty() == "etl.bool" {
				if b, has := extractBool(instanceData, key); has {
					if err = metric.SetValueBool(instance, b); err != nil {
						r.Logger.Error().Msgf("SetValueBool [metric=%s]: %v", key, err)
					}
					count++
				}
			} else if metric.GetProperty() == "etl.float" {
				if f, has := extractFloat(instanceData, key); has {
					if err = metric.SetValueFloat64(instance, f); err != nil {
						r.Logger.Error().Msgf("SetValueFloat64 [metric=%s]: %v", key, err)
					}
					count++
				}
			}
		}
	}

	r.Logger.Info().Msgf("collected %d data points (api time = %s) (parse time = %s)", count, apiD.String(), parseD.String())

	r.Metadata.LazySetValueInt64("api_time", "data", apiD.Microseconds())
	r.Metadata.LazySetValueInt64("parse_time", "data", parseD.Microseconds())
	r.Metadata.LazySetValueUint64("count", "data", count)
	r.AddCollectCount(count)

	return r.Matrix, nil
}

// these functions are highly inefficient/expensive, only for prototyping
func extractLabel(data map[string]interface{}, path string) (string, bool) {
	if x := strings.Split(path, "."); len(x) > 1 {
		if deeper, ok := data[x[0]].(map[string]interface{}); ok {
			return extractLabel(deeper, strings.Join(x[1:], "."))
		}
		return "", false // path not found
	}
	v, ok := data[path].(string)
	return v, ok
}

func extractBool(data map[string]interface{}, path string) (bool, bool) {
	if x := strings.Split(path, "."); len(x) > 1 {
		if deeper, ok := data[x[0]].(map[string]interface{}); ok {
			return extractBool(deeper, strings.Join(x[1:], "."))
		}
		return false, false // path not found
	}
	v, ok := data[path].(bool)
	return v, ok
}

func extractFloat(data map[string]interface{}, path string) (float64, bool) {
	if x := strings.Split(path, "."); len(x) > 1 {
		if deeper, ok := data[x[0]].(map[string]interface{}); ok {
			return extractFloat(deeper, strings.Join(x[1:], "."))
		}
		return 0, false // path not found
	}
	v, ok := data[path].(float64)
	return v, ok
}
