//package prometheusFileSd
package main

import (
	"fmt"
	"goharvest2/pkg/conf"
	"goharvest2/pkg/util"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type prometheusFileSdConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

type pollerFileSdConfig struct {
	pollerName string
	addr       string
}

var portMap = make(map[int]int)

var filePath = path.Join(conf.GetHarvestHomePath(), "filesd.yml")

var promPollerPortMap map[string]int

var existingPollersInFileSd []string

var promFileSdConfig []prometheusFileSdConfig

var pollerNames []string

func init() {
	// load existing pollers for which filesd is configured
	promFileSdConfig = load()
	if len(promFileSdConfig) == 0 {
		fmt.Println("file sd doesnt exist")
	}
	promPollerPortMap = buildPromPollerToPortMapping(promFileSdConfig)
}

func RefreshPrometheusSdConfig(harvestConfPath string) {
	var err error
	pollerNames, err = conf.GetPollerNames(harvestConfPath)
	pollerConfig := getPollersFileSdConfig(&pollerNames, harvestConfPath)
	if err != nil {

	}
	// check if file exists
	if !util.CheckFileExists(filePath) {
		for _, p := range pollerConfig {
			populate(&promFileSdConfig, &p)
		}
		fmt.Println(promFileSdConfig)
		writeToFile(&promFileSdConfig)
	} else {
		// Build label to portname mapping
		fmt.Println(promPollerPortMap)

		pollerAdded, pollerDeleted := pollerDiff(&promPollerPortMap, &pollerConfig)
		fmt.Println("--------added--------")
		fmt.Println(pollerAdded)
		fmt.Println("--------deleted--------")
		fmt.Println(pollerDeleted)

		if len(pollerAdded) > 0 || len(pollerDeleted) > 0 {
			fmt.Println("config changed pls reload your prom yaml file")
		}
		// if pollers were added
		// update existing config
		for _, pollerConfig := range pollerAdded {
			populate(&promFileSdConfig, &pollerConfig)
		}

		fmt.Println(promFileSdConfig)

		// if pollers were deleted
		// update existing config
		for _, pollerConfig := range pollerDeleted {
			index := findIndex(&promFileSdConfig, pollerConfig.pollerName)
			promFileSdConfig = removeIndex(&promFileSdConfig, index)
		}

		fmt.Println(promFileSdConfig)

		for i := range promFileSdConfig {
			//fmt.Println("----before---")
			//fmt.Println(config[i])
			validateConfigForPort(&promFileSdConfig[i])
			//fmt.Println("----after---")
			//fmt.Println(config[i])
		}
		fmt.Println(promFileSdConfig)

		writeToFile(&promFileSdConfig)
	}

}

func writeToFile(promFileSdConfig *[]prometheusFileSdConfig) {
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
	//fmt.Println(string(contents))
	err = yaml.Unmarshal(contents, &config)
	if err != nil {
		fmt.Println(err)
	}
	//fmt.Println(config)
	return config
}

func main() {
	RefreshPrometheusSdConfig("/home/rahulg2/code/github/harvest/harvest.yml")
}

func validateConfigForPort(fileSdConfig *prometheusFileSdConfig) {
	addrPort := strings.Split(fileSdConfig.Targets[0], ":")
	port := util.LastString(addrPort)
	addr := addrPort[0]
	if !checkPortAvailable(addr, port) {
		// update config
		newPort, _, err := GetDistinctPort()
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("changing port from %s to %d \n", port, newPort)
		(*fileSdConfig).Targets = []string{"localhost:" + strconv.Itoa(newPort)}
	}

}

func checkPortAvailable(addr string, port string) bool {
	ln, err := net.Listen("tcp", addr+":"+port)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't listen on port %q: %s", port, err)
		return false
	}

	ln.Close()
	fmt.Printf("TCP Port %q is available", port)
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

	fmt.Println("----------------")
	fmt.Println(pollerNamesMap)
	fmt.Println(promPollerPortMap)
	fmt.Println("#################")
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

func buildPromPollerToPortMapping(config []prometheusFileSdConfig) map[string]int {
	var pollerPortMap = make(map[string]int)
	for _, fileSdConfig := range config {
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

	port, _, err := GetDistinctPort()
	if err != nil {
		fmt.Println(err)
	}

	fileSdConfig := prometheusFileSdConfig{[]string{p.addr + ":" + strconv.Itoa(port)}, m}
	*fileSdConfigs = append(*fileSdConfigs, fileSdConfig)
}

func GetDistinctPort() (int, int, error) {
	retries := 50
	for i := 0; i < retries; i++ {
		port, err := util.GetFreePort()
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
