package main

import (
    "encoding/json"
    "goharvest2/cmd/harvest/version"
    "goharvest2/pkg/conf"
    "goharvest2/pkg/matrix"
    "goharvest2/pkg/tree/node"
    "io/ioutil"
    "os/exec"
    "strconv"
    "strings"
)

type telemetryMessage struct {
    Target *targetInfo
    Nodes *instanceInfo
    SVMs *instanceInfo
    Volumes *instanceInfo
    Platform *platformInfo
    Harvest *harvestInfo
}

type targetInfo struct {
    Version string
    Model string
    Ping float32
}

type instanceInfo struct {
    Count int64
    DataPoints int64
    PollTime int64
    ApiTime int64
    ParseTime int64
    PluginTime int64
}

type platformInfo struct {
    OS string
    Arch string
    MemoryKb uint64
    CPUs uint8
}

type harvestInfo struct {
    Version string
    Release string
    Commit string
    BuildDate string
    NumClusters uint8
}

func (p *Poller) SendTelemetry() (*matrix.Matrix, error) {

    var (
        msg *telemetryMessage
        msgJson []byte
        arch string
        cpus uint8
        err error
    )

    // @DEBUG all log messages are info or higher, only for development/debugging
    logger.Info().Msg("collecting data for telemetry")

    msg = new(telemetryMessage)

    // add harvest release info
    msg.Harvest = new(harvestInfo)
    msg.Harvest.Version = version.VERSION
    msg.Harvest.Release = version.Release
    msg.Harvest.Commit = version.Commit
    msg.Harvest.BuildDate = version.BuildDate
    msg.Harvest.NumClusters = getNumClusters(p.options.Config)

    // add info about platform (where Harvest is running)
    msg.Platform = new(platformInfo)
    arch, cpus = getCPUInfo()
    msg.Platform.Arch = arch
    msg.Platform.CPUs = cpus
    msg.Platform.MemoryKb = getRamSize()
    msg.Platform.OS = getOSName()

    // info about ONTAP host and instances
    msg.Target, msg.Nodes, msg.SVMs, msg.Volumes = p.getInstanceInfo()

    if msgJson, err = json.MarshalIndent(msg, "", "  "); err != nil {
        logger.Error().Stack().Err(err).Msg("marshal msg")
        return nil, err
    }

    logger.Info().Msgf("composed telemetry message:\n%s\n", string(msgJson))

    // @TODO send as ASUP message

    return nil, nil
}

func (p *Poller) getInstanceInfo() (*targetInfo, *instanceInfo, *instanceInfo, *instanceInfo) {
    target := new(targetInfo)
    nodes := new(instanceInfo)
    svms := new(instanceInfo)
    vols := new(instanceInfo)

    // get ping value from poller metadata
    target.Ping, _ = p.status.LazyGetValueFloat32("ping", "host")

    // scan collectors

    for _, c := range p.collectors {

        if c.GetName() == "ZapiPerf" {

            if c.GetObject() == "SystemNode" {
                md := c.GetMetadata()
                nodes.Count, _ = md.LazyGetValueInt64("count", "instance")
                nodes.DataPoints, _ = md.LazyGetValueInt64("count", "data")
                nodes.PollTime, _ = md.LazyGetValueInt64("poll_time", "data")
                nodes.ApiTime, _ = md.LazyGetValueInt64("api_time", "data")
                nodes.ParseTime, _ = md.LazyGetValueInt64("parse_time", "data")
                nodes.PluginTime, _ = md.LazyGetValueInt64("plugin_time", "data")

                target.Version, target.Model = c.GetHostInfo()

            } else if c.GetObject() == "Volume" {
                md := c.GetMetadata()
                vols.Count, _ = md.LazyGetValueInt64("count", "instance")
                vols.DataPoints, _ = md.LazyGetValueInt64("count", "data")
                vols.PollTime, _ = md.LazyGetValueInt64("poll_time", "data")
                vols.ApiTime, _ = md.LazyGetValueInt64("api_time", "data")
                vols.ParseTime, _ = md.LazyGetValueInt64("parse_time", "data")
                vols.PluginTime, _ = md.LazyGetValueInt64("plugin_time", "data")

            } else if c.GetObject() == "CIFSvserver" {
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

    return target, nodes, svms, vols
}


func getCPUInfo() (string, uint8) {

    var (
        arch, countString, line string
        fields []string
        count int
        output []byte
        err error
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
        if count, err = strconv.Atoi(countString); err != nil {
        }
    }

    return arch, uint8(count)
}

func getRamSize() uint64 {

    var (
        output []byte
        err error
        line, sizeString string
        fields []string
        size uint64
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
        output []byte
        err error
        name, line string
        fields []string
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

func getNumClusters(configFp string) uint8 {
    var (
        count uint8
        pollers, poller, collectors *node.Node
        collectorName string
        err error
    )

    if pollers, err = conf.GetPollers(configFp); err == nil {
        for _, poller = range pollers.GetChildren() {
            if collectors = poller.GetChildS("collectors"); collectors != nil {
                for _, collectorName = range collectors.GetAllChildContentS() {
                    if collectorName == "Zapi" || collectorName == "ZapiPerf" {
                        count++
                        break
                    }
                }
            }
        }
    }

    return count
}
