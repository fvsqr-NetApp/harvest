package rest

import (
	"crypto/tls"
	"encoding/json"
	"goharvest2/pkg/errors"
	"goharvest2/pkg/logging"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	httpc   *http.Client
	request *http.Request
	baseUrl string
	cluster Cluster
	Logger  *logging.Logger
}

type Cluster struct {
	name    string
	info    string
	uuid    string
	version [3]int
}

func New(addr string, certAuth, basicAuth *[2]string, useInsecureTls bool) (*Client, error) {

	var (
		c Client
		//clientTimeout time.Duration
		err error
	)

	c = Client{baseUrl: "https://" + addr + "/api/"}
	c.Logger = logging.SubLogger("Rest", "Client")

	c.Logger.Info().Msgf("using base URL: %v", c.baseUrl)

	if c.request, err = http.NewRequest("GET", c.baseUrl, nil); err != nil {
		return nil, err
	}

	c.request.Header.Set("Accept", "application/json")

	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: useInsecureTls},
	}

	if certAuth != nil {
		if cert, err := tls.LoadX509KeyPair(certAuth[0], certAuth[1]); err == nil {
			transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
		} else {
			return nil, err
		}
	} else if basicAuth != nil {
		c.request.SetBasicAuth(basicAuth[0], basicAuth[1])
	} else {
		return nil, errors.New(errors.MISSING_PARAM, "certAuth or basicAuth")
	}

	c.httpc = &http.Client{Transport: transport, Timeout: time.Duration(10) * time.Second}

	return &c, nil
}

func (c *Client) Init(retries int) error {

	var (
		err           error
		content       []byte
		data, version map[string]interface{}
		s             string
		i, k          int
		f             float64
		ok            bool
	)

	for i = 0; i < retries; i++ {
		if content, err = c.Get("cluster", nil); err != nil {
			continue
		}

		if err = json.Unmarshal(content, &data); err != nil {
			return err
		}

		if s, ok = data["name"].(string); ok {
			c.cluster.name = s
		}

		if s, ok = data["uuid"].(string); ok {
			c.cluster.uuid = s
		}

		if version, ok = data["version"].(map[string]interface{}); ok {
			if s, ok = version["full"].(string); ok {
				c.cluster.info = s
			}

			for k, s = range [3]string{"generation", "major", "minor"} {
				if f, ok = version[s].(float64); ok {
					c.Logger.Debug().Msgf("OK - parsing [%s] (%T): %v", s, version[s], version[s])
					c.cluster.version[k] = int(f)
				}
			}
		}

		return nil
	}

	return err
}

func (c *Client) ClusterName() string {
	return c.cluster.name
}

func (c *Client) ClusterUUID() string {
	return c.cluster.uuid
}

func (c *Client) Info() string {
	return c.cluster.info

}

func (c *Client) Version() [3]int {
	return c.cluster.version
}

func (c *Client) Get(path string, params map[string]string) ([]byte, error) {
	var (
		response *http.Response
		fullUrl  string
		err      error
	)

	fullUrl = c.baseUrl + path + "?"

	// add params to url
	if params == nil {
		params = make(map[string]string)
	}

	if _, has := params["return_records"]; !has {
		params["return_records"] = "true"
	}

	if _, has := params["return_timeout"]; !has {
		params["return_timeout"] = "15"
	}

	for k, v := range params {
		if strings.HasSuffix(fullUrl, "?") {
			fullUrl += k + "=" + v
		} else {
			fullUrl += "&" + k + "=" + v
		}
	}

	if c.request.URL, err = url.Parse(fullUrl); err != nil {
		return nil, err
	}
	c.Logger.Info().Msgf("using URL: %s", c.request.URL.String())

	if response, err = c.httpc.Do(c.request); err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, errors.New(errors.API_RESPONSE, response.Status)
	}

	return ioutil.ReadAll(response.Body)
}
