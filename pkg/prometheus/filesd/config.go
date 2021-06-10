package filesd

//package main

import (
	"fmt"
	"goharvest2/pkg/conf"
	"goharvest2/pkg/util"
	"io/ioutil"
	"net"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	portMap            = make(map[int]int)                                // Used for tracking unique ports generated through GetDistinctPort
	fileSdName         = "promfilesd.yml"                                 // Name of file for filesd configuration
	filePath           = path.Join(conf.GetHarvestHomePath(), fileSdName) // Absolute path of fileSdName
	pollerPortMap      map[string]int                                     // Contains existing mapping of poller and port in filesd defined at filePath
	promFileSdConfig   []prometheusFileSdConfig                           // data mapping from file promFileSdConfig
	runningPollersPort map[string]string                                  // Defines current port mapping of running pollers if any
)

//prometheusFileSdConfig used for marshaling and unmarshalling file fileSdName
type prometheusFileSdConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

//pollerFileSdConfig Used for building prometheusFileSdConfig
type pollerFileSdConfig struct {
	pollerName string
	addr       string
}

//RefreshPrometheusSdConfig This method refreshes fileSdName with current harvest configuration
func RefreshPrometheusSdConfig(harvestConfPath string, mapRunningPollersPromPort map[string]string) {
	//assign currently running pollers port to runningPollersPort
	runningPollersPort = mapRunningPollersPromPort
	//get all pollernames from harvest config
	allPollerNames, err := conf.GetPollerNames(harvestConfPath)
	if err != nil {
		fmt.Printf("Error while reading pollernames from harvest config %v \n", err)
	}
	//read pollers which have filesd enabled from harvest config
	pollerConfig := getPollersFileSdConfig(&allPollerNames, harvestConfPath)

	// check if file exists
	if !util.CheckFileExists(filePath) {
		// if file doesn't exist
		for _, p := range pollerConfig {
			populate(&promFileSdConfig, &p)
		}
		// write to fileSdName
		writeToFile(&promFileSdConfig, filePath)
	} else {
		// if filesd exists then load existing filesd configuration from fileSdName
		promFileSdConfig = load()
		// build current pollerport mapping from filesdconfiguration
		pollerPortMap = buildPromPollerToPortMapping(&promFileSdConfig)
		// identify if any pollers are added or deleted
		pollerAdded, pollerDeleted := pollerDiff(&pollerPortMap, &pollerConfig)

		// if pollers were added
		// update existing config
		for _, pollerConfig := range pollerAdded {
			populate(&promFileSdConfig, &pollerConfig)
		}

		// if pollers were deleted
		// update existing config
		for _, pollerConfig := range pollerDeleted {
			index := findIndex(&promFileSdConfig, pollerConfig.pollerName)
			promFileSdConfig = removeIndex(&promFileSdConfig, index)
		}

		for i := range promFileSdConfig {
			validateConfigForPort(&promFileSdConfig[i])
		}

		// if there are any pollers added/deleted then print the message
		if len(pollerAdded) > 0 || len(pollerDeleted) > 0 {
			fmt.Printf("config changed pls reload your %s file", filePath)
		}
		writeToFile(&promFileSdConfig, filePath)
	}
	pollerPortMap = buildPromPollerToPortMapping(&promFileSdConfig)

}

func writeToFile(promFileSdConfig *[]prometheusFileSdConfig, filePath string) {
	if len(*promFileSdConfig) > 0 || util.CheckFileExists(filePath) {
		b, _ := yaml.Marshal(&promFileSdConfig)
		ioutil.WriteFile(filePath, b, 0644)
	}
}

func load() []prometheusFileSdConfig {
	var config []prometheusFileSdConfig
	if !util.CheckFileExists(filePath) {
		return config
	}
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("error reading config file=[%s] %+v\n", filePath, err)
	}
	err = yaml.Unmarshal(contents, &config)
	if err != nil {
		fmt.Println(err)
	}
	return config
}

func main() {
	harvestConfPath := "/home/rahulg2/code/github/harvest/harvest.yml"
	pollerNames, _ := conf.GetPollerNames(harvestConfPath)
	var mapPollerNamesStatus map[string]string
	for _, p := range pollerNames {
		mapPollerNamesStatus[p] = "not running"
	}
	RefreshPrometheusSdConfig(harvestConfPath, mapPollerNamesStatus)
	fmt.Println(GetPromport("umeng_aff300"))
}

func validateConfigForPort(fileSdConfig *prometheusFileSdConfig) {
	addrPort := strings.Split(fileSdConfig.Targets[0], ":")
	port := util.LastString(addrPort)
	addr := addrPort[0]
	if !checkPortAvailable(addr, port) {
		// update config
		newPort, _, err := GetDistinctPort(addr)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("changing port from %s to %d \n", port, newPort)
		(*fileSdConfig).Targets = []string{addr + strconv.Itoa(newPort)}
	}

}

func checkPortAvailable(addr string, port string) bool {
	ln, err := net.Listen("tcp", addr+":"+port)

	if err != nil {
		//fmt.Fprintf(os.Stderr, "Can't listen on port %q: %s", port, err)
		return false
	}

	ln.Close()
	//fmt.Printf("TCP Port %q is available", port)
	return true
}

func findIndex(s *[]prometheusFileSdConfig, pollerName string) int {
	for i, config := range *s {
		if config.Labels["job"] == pollerName {
			return i
		}
	}
	return -1
}

func removeIndex(s *[]prometheusFileSdConfig, index int) []prometheusFileSdConfig {
	return append((*s)[:index], (*s)[index+1:]...)
}

func pollerDiff(promPollerPortMap *map[string]int, pollerConfig *[]pollerFileSdConfig) ([]pollerFileSdConfig, []pollerFileSdConfig) {

	var pollerDeleted []pollerFileSdConfig
	var pollerAdded []pollerFileSdConfig

	var pollerNamesMap = map[string]pollerFileSdConfig{}
	for _, p := range *pollerConfig {
		pollerNamesMap[p.pollerName] = p
	}

	// calculated pollers added
	for pollerName, value := range pollerNamesMap {
		if _, ok := (*promPollerPortMap)[pollerName]; !ok {
			// if poller doesn't exist in map then it's newly added
			pollerAdded = append(pollerAdded, value)
		}
	}

	// calculate poller deleted
	for pollerName, _ := range *promPollerPortMap {
		if _, ok := pollerNamesMap[pollerName]; !ok {
			// if poller doesn't exist in map then it's deleted
			pollerDeleted = append(pollerDeleted, pollerFileSdConfig{pollerName: pollerName})
		}
	}
	return pollerAdded, pollerDeleted
}

func buildPromPollerToPortMapping(config *[]prometheusFileSdConfig) map[string]int {
	var pollerPortMap = make(map[string]int)
	for _, fileSdConfig := range *config {
		labels := fileSdConfig.Labels
		targets := fileSdConfig.Targets

		// pick first values only as we populate only 1 value
		pollerName := labels["job"]
		port := util.LastString(strings.Split(targets[0], ":"))
		pollerPortMap[pollerName], _ = strconv.Atoi(port)
	}
	return pollerPortMap
}

func populate(fileSdConfigs *[]prometheusFileSdConfig, p *pollerFileSdConfig) {
	m := map[string]string{ // Map literal
		"job": p.pollerName,
	}
	var port int
	var err error

	fmt.Println(runningPollersPort)
	if val, ok := runningPollersPort[p.pollerName]; ok {
		po, err := strconv.Atoi(val)
		if err != nil {
			fmt.Printf("error while parsing port for running poller %s %s %v \n", p.pollerName, val, err)
		}
		port = po
	} else {
		port, _, err = GetDistinctPort(p.addr)
		if err != nil {
			fmt.Println(err)
		}
	}

	fileSdConfig := prometheusFileSdConfig{[]string{p.addr + ":" + strconv.Itoa(port)}, m}
	*fileSdConfigs = append(*fileSdConfigs, fileSdConfig)
}

func GetDistinctPort(addr string) (int, int, error) {
	retries := 50
	for i := 0; i < retries; i++ {
		port, err := util.GetFreePort(addr)
		if err != nil {
			return 0, i, err
		}
		if _, ok := portMap[port]; !ok {
			portMap[port] = port
			return port, i, nil
		}
	}
	return -1, retries, fmt.Errorf("can't find distinct free port on system after %d retries", retries)
}

//getPollersFileSdConfig read pollers from harvest config which have filesd enabled
func getPollersFileSdConfig(pollerNames *[]string, configPath string) []pollerFileSdConfig {
	if conf.Config == (conf.HarvestConfig{}) {
		conf.LoadHarvestConfig(configPath)
	}
	var p []pollerFileSdConfig
	for _, pollerName := range *pollerNames {
		if (*conf.Config.Pollers)[pollerName] != (conf.Poller{}) {
			exporters := (*conf.Config.Pollers)[pollerName].Exporters
			for _, exporterName := range *exporters {
				if (*conf.Config.Exporters)[exporterName] != (conf.Exporter{}) {
					exporterType := (*conf.Config.Exporters)[exporterName].Type
					fileSd := (*conf.Config.Exporters)[exporterName].FileSd
					if exporterType != nil && *exporterType == "Prometheus" && fileSd != nil && *fileSd.Enabled {
						addr := (*conf.Config.Exporters)[exporterName].Addr
						p = append(p, pollerFileSdConfig{pollerName: pollerName, addr: *addr})
					}
				}
			}
		}
	}
	return p
}

func GetPromport(pollerName string) int {
	return pollerPortMap[pollerName]
}
