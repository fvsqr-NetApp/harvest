package asup

import (
	"encoding/json"
	"fmt"
	"goharvest2/cmd/harvest/version"
	"goharvest2/cmd/poller/collector"
	"goharvest2/pkg/errors"
	"goharvest2/pkg/matrix"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type asupMessage struct {
	Target   *targetInfo
	Nodes    *instanceInfo
	Volumes  *instanceInfo
	Svms     *instanceInfo
	Platform *platformInfo
	Harvest  *harvestInfo
}

type targetInfo struct {
	Version string
	Model   string
	Serial  string
	Ping    float64
}

type instanceInfo struct {
	Count      int64
	DataPoints int64
	PollTime   int64
	ApiTime    int64
	ParseTime  int64
	PluginTime int64
}

type platformInfo struct {
	OS       string
	Arch     string
	MemoryKb uint64
	CPUs     uint8
}

type harvestInfo struct {
	UUID        string
	Version     string
	Release     string
	Commit      string
	BuildDate   string
	NumClusters uint8
}

func DoAsupMessage(collectors []collector.Collector, status *matrix.Matrix, harvestUUID string) error {

	var (
		msg *asupMessage
		err error
	)

	if msg, err = buildAsupMessage(collectors, status, harvestUUID); err != nil {
		return errors.New(errors.ERR_CONFIG, "failed to build ASUP message")
	}

	if err = sendAsupMessage(msg); err != nil {
		return errors.New(errors.ERR_CONFIG, "failed to send ASUP message")
	}

	return nil
}

// TODO: This function will be used to invoke harvest-asup(private repo)
func sendAsupMessage(msg *asupMessage) error {
	file, err := os.OpenFile("payload.json", os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		errors.New(errors.ERR_CONFIG, "asup json creation failed")
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(msg); err != nil {
		errors.New(errors.ERR_CONFIG, "writing to payload failed")
	}

	//fmt.Sprintf("%#v", msg)
	return nil
}

func buildAsupMessage(collectors []collector.Collector, status *matrix.Matrix, harvestUUID string) (*asupMessage, error) {

	var (
		msg  *asupMessage
		arch string
		cpus uint8
	)

	// @DEBUG all log messages are info or higher, only for development/debugging
	fmt.Print("building ASUP message")

	msg = new(asupMessage)

	// add harvest release info
	msg.Harvest = new(harvestInfo)
	msg.Harvest.UUID = harvestUUID
	msg.Harvest.Version = version.VERSION
	msg.Harvest.Release = version.Release
	msg.Harvest.Commit = version.Commit
	msg.Harvest.BuildDate = version.BuildDate
	msg.Harvest.NumClusters = getNumClusters(collectors)

	// add info about platform (where Harvest is running)
	msg.Platform = new(platformInfo)
	arch, cpus = getCPUInfo()
	msg.Platform.Arch = arch
	msg.Platform.CPUs = cpus
	msg.Platform.MemoryKb = getRamSize()
	msg.Platform.OS = getOSName()

	// info about ONTAP host and instances
	msg.Target, msg.Nodes, msg.Volumes, msg.Svms = getInstanceInfo(collectors, status)

	return msg, nil
}

func getInstanceInfo(collectors []collector.Collector, status *matrix.Matrix) (*targetInfo, *instanceInfo, *instanceInfo, *instanceInfo) {
	target := new(targetInfo)
	nodes := new(instanceInfo)
	vols := new(instanceInfo)
	svms := new(instanceInfo)

	// get ping value from poller metadata
	target.Ping, _ = status.LazyGetValueFloat64("ping", "host")

	// scan collectors

	for _, c := range collectors {

		if c.GetName() == "Zapi" {

			if c.GetObject() == "Node" {
				md := c.GetMetadata()
				nodes.Count, _ = md.LazyGetValueInt64("count", "instance")
				nodes.DataPoints, _ = md.LazyGetValueInt64("count", "data")
				nodes.PollTime, _ = md.LazyGetValueInt64("poll_time", "data")
				nodes.ApiTime, _ = md.LazyGetValueInt64("api_time", "data")
				nodes.ParseTime, _ = md.LazyGetValueInt64("parse_time", "data")
				nodes.PluginTime, _ = md.LazyGetValueInt64("plugin_time", "data")

				target.Version = c.GetHostVersion()
				target.Model = c.GetHostModel()
				target.Serial = c.GetHostUUID()

			} else if c.GetObject() == "Volume" {
				md := c.GetMetadata()
				vols.Count, _ = md.LazyGetValueInt64("count", "instance")
				vols.DataPoints, _ = md.LazyGetValueInt64("count", "data")
				vols.PollTime, _ = md.LazyGetValueInt64("poll_time", "data")
				vols.ApiTime, _ = md.LazyGetValueInt64("api_time", "data")
				vols.ParseTime, _ = md.LazyGetValueInt64("parse_time", "data")
				vols.PluginTime, _ = md.LazyGetValueInt64("plugin_time", "data")

				//} else if c.GetObject() == "Svm" {
				//	md := c.GetMetadata()
				//	svms.Count, _ = md.LazyGetValueInt64("count", "instance")
				//	svms.DataPoints, _ = md.LazyGetValueInt64("count", "data")
				//	svms.PollTime, _ = md.LazyGetValueInt64("poll_time", "data")
				//	svms.ApiTime, _ = md.LazyGetValueInt64("api_time", "data")
				//	svms.ParseTime, _ = md.LazyGetValueInt64("parse_time", "data")
				//	svms.PluginTime, _ = md.LazyGetValueInt64("plugin_time", "data")
			}
		} else if c.GetName() == "ZapiPerf" {

			if c.GetObject() == "Svm" {
				md := c.GetMetadata()
				svms.Count, _ = md.LazyGetValueInt64("count", "instance")
				svms.DataPoints, _ = md.LazyGetValueInt64("count", "data")
				svms.PollTime, _ = md.LazyGetValueInt64("poll_time", "data")
				svms.ApiTime, _ = md.LazyGetValueInt64("api_time", "data")
				svms.ParseTime, _ = md.LazyGetValueInt64("parse_time", "data")
				svms.PluginTime, _ = md.LazyGetValueInt64("plugin_time", "data")
			}
		}
	}

	return target, nodes, vols, svms
}

func getCPUInfo() (string, uint8) {

	var (
		arch, countString, line string
		fields                  []string
		count                   uint64
		output                  []byte
		err                     error
	)

	if output, err = exec.Command("lscpu").Output(); err == nil {
		for _, line = range strings.Split(string(output), "\n") {
			if fields = strings.Fields(line); len(fields) >= 2 {
				if fields[0] == "Architecture:" {
					arch = fields[1]
				} else if fields[0] == "CPU(s):" {
					countString = fields[1]
				}
			}
		}
	}

	if countString != "" {
		if count, err = strconv.ParseUint(countString, 10, 8); err != nil {
		}
	}

	return arch, uint8(count)
}

func getRamSize() uint64 {

	var (
		output           []byte
		err              error
		line, sizeString string
		fields           []string
		size             uint64
	)

	if output, err = exec.Command("free", "--kilo").Output(); err == nil {
		for _, line = range strings.Split(string(output), "\n") {
			if fields = strings.Fields(line); len(fields) >= 4 && fields[0] == "Mem:" {
				sizeString = fields[1]
				break
			}
		}
	}

	if sizeString != "" {
		if size, err = strconv.ParseUint(sizeString, 10, 64); err != nil {
			size = 0
		}
	}

	return size
}

func getOSName() string {

	var (
		output     []byte
		err        error
		name, line string
		fields     []string
	)

	if output, err = ioutil.ReadFile("/etc/os-release"); err == nil {
		for _, line = range strings.Split(string(output), "\n") {
			if fields = strings.SplitN(line, "=", 2); len(fields) == 2 {
				if fields[0] == "NAME" {
					name = fields[1]
				} else if fields[1] == "PRETTY_NAME" {
					name = fields[1]
					break
				}
			}
		}
	}
	return strings.Trim(name, `"`)
}

func getNumClusters(collectors []collector.Collector) uint8 {
	var count uint8

	for _, collector := range collectors {
		if collector.GetName() == "Zapi" || collector.GetName() == "ZapiPerf" {
			count++
			break
		}
	}

	return count
}

//func Work(status *matrix.Matrix, collectors []collector.Collector) (*matrix.Matrix, error) {
//	if collectors != nil {
//		if err := doAsupMessage(collectors, status); err != nil {
//			return nil, err
//		}
//	}
//	return nil, nil
//}
