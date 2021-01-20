package main

import (
	"runtime"
	"errors"
	"sync"
	"os"
	//"os/signal"
	"syscall"
	"strconv"
	"path"
	"local.host/logger"
	"local.host/schedule"
	"local.host/collector"
	"local.host/exporter"
	"local.host/args"
	"local.host/yaml"
)

var Log *logger.Logger = logger.New(1, "")

/*
var SIGNALS = []os.Signal{
			syscall.SIGHUP, 
			syscall.SIGINT, 
			syscall.SIGTERM,
			syscall.SIGQUIT,
}*/

type Poller struct {
	Name string
	Args *args.Args
	pid int
	pidf string
	schedule *schedule.Schedule
	collectors []*collector.Collector
	exporters []*exporter.Exporter
	//metadata *metadata.Metadata
}

func New() *Poller {
	return &Poller{}
}

func (p *Poller) Init() error {

	var err error
	/* Set poller main attributes */
	p.Args, p.Name, err = args.ReadArgs()

	/* If daemon, make sure handler outputs to file */
	if p.Args.Daemon {
		err := logger.OpenFileOutput(p.Args.Path, "harvest_poller_" + p.Name + ".log")
		if err != nil {
			return err
		}
	}
	Log = logger.New(p.Args.LogLevel, p.Name)

	/* Useful info for debugging */
	if p.Args.Debug {
		p.LogDebugInfo()
	}

	/* Set signal handler */
	/*
	signal_channel := make(chan os.Signal, 1)
	signal.Notify(signal_channel, SIGNALS...)
	go p.handleSignals(signal_channel)
	Log.Debug("Set signal handler for [%v]", SIGNALS)
	*/

	/* Write PID to file */ 
	err = p.registerPid()
	if err != nil {
		Log.Warn("Failed to write PID file: %v", err)
	}

	/* Announce startup */
	if p.Args.Daemon {
		Log.Info("Starting as daemon [pid=%d] [pid file=%s]", p.pid, p.pidf)
	} else {
		Log.Info("Starting in foreground [pid=%d] [pid file=%s]", p.pid, p.pidf)
	}

	/* Set Harvest API handler */
	go p.handleFifo()

	/* Initialize exporters and collectors */
	params, exporters, err := ReadConfig(p.Args.Path, p.Args.Config, p.Name)
	if err != nil {
		Log.Error("Failed to read config: %v", err)
		return err
	}

	collectors := params.PopChild("collectors")
	if collectors == nil {
		Log.Warn("No collectors defined for poller")
		return errors.New("No collectors")
	}
	p.exporters = make([]*exporter.Exporter, 0)
	p.initExporters(exporters)
	Log.Debug("Initialized %d exporters", len(p.exporters))

	p.collectors = make([]*collector.Collector, 0)
	p.initCollectors(collectors, params)
	Log.Debug("Initialized %d collectors", len(p.collectors))

	/* Set up our own schedule */
	interval, err := strconv.Atoi(params.GetChildValue("poller_interval"))
	if err != nil || interval <= 0 {
		interval = 20
		Log.Debug("Using default interval")
	}
	p.schedule = schedule.New(interval)

	/* Famous last words */
	Log.Info("Poller start-up complete. Set monitoring interval [%d s]", interval)

	return nil

}

func (p *Poller) initExporters(exporter_params *yaml.Node) {
	for _, params := range exporter_params.Children {
		exp := exporter.New(params)
		err := exp.Init()
		if err != nil {
			Log.Error("Failed initializing Exporter [%s]: %v", params.Name, err)
		} else {
			Log.Debug("Initialized Exporter [%s]", params.Name)
			p.addExporter(exp)
		}
	}
}

func (p *Poller) initCollectors(collectors, params *yaml.Node) {
	for _, class := range collectors.Values {
		col := collector.New(class, params, p.Args)
		err := col.Init()

		if err != nil {
			Log.Error("Failed initializing Collector [%s]: %v", class, err)
		} else if len(col.SubCollectors) > 0 {
			Log.Debug("Initialized Collector [%s] with %d subcollectors", class, len(col.SubCollectors))
			for _, sub := range col.SubCollectors {
				p.addCollector(sub)
				wanted_exporters := sub.Params.PopChild("exporters")
				for _, exporter_name := range wanted_exporters.Values {
					exp := p.getExporter(exporter_name)
					if exp == nil {
						Log.Warn("Exporter [%s] requested by [%s:%s]", exporter_name, sub.Class, sub.Name)
					} else {
						sub.Exporters = append(sub.Exporters, exp)
						Log.Warn("Attached Exporter [%s] to [%s:%s]", exporter_name, sub.Class, sub.Name)
					}
				}
			}
		} else {
			p.addCollector(col)
		}
	}
}

func (p *Poller) addExporter(e *exporter.Exporter) {
	p.exporters = append(p.exporters, e)
}

func (p *Poller) addCollector(c *collector.Collector) {
	p.collectors = append(p.collectors, c)
}

func (p *Poller) getExporter(name string) *exporter.Exporter {
	for _, exp := range p.exporters {
		if exp.Name == name {
			return exp
		}
	}
	return nil
}

func (p *Poller) Start() {

	var wg sync.WaitGroup

	/* Start collectors */
	for _, col := range p.collectors {
		Log.Debug("Starting collector [%s]", col.Name)
		wg.Add(1)
		go col.Start(&wg)
	}

	go p.selfMonitor()

	wg.Wait()

	Log.Info("No active collectors. Poller terminating.")
	p.cleanup()
	os.Exit(0)
}

func (p *Poller) cleanup() {
	Log.Info("Cleaning up and stopping Poller [pid=%d]", p.pid)

	if p.Args.Daemon {

		var err error

		err = os.Remove(p.pidf)
		if err != nil {
			Log.Warn("Failed to clean pid file: %v", err)
		} else {
			Log.Debug("Clean pid file [%s]", p.pidf)
		}

		err = logger.CloseFileOutput()
		if err != nil {
			Log.Error("Failed to close log file: %v", err)
		}
	}
}


func (p *Poller) selfMonitor() {

	for {
		p.schedule.Start()

		Log.Info("Updated status of %d collectors and %d exporters.", len(p.collectors), len(p.exporters))

		t := p.schedule.Pause()
		if t < 0 {
			Log.Warn("Lagging behind schedule %s", t.String())
		}

	}
}

/*
func (p *Poller) handleSignals(signal_channel chan os.Signal) {
	for {
		sig := <-signal_channel
		Log.Info("Caught signal [%s]", sig)
		p.cleanup()
		os.Exit(0)
	}
}*/ 

func (p *Poller) handleFifo() {
	Log.Info("Serving APIs for Harvest2 daemon")
	for {
		;
	}
}

func (p *Poller) registerPid() error {
	var err error
	p.pid = os.Getpid()
	if p.Args.Daemon {
		var file *os.File
		p.pidf = path.Join(p.Args.Path, "var", p.Name + ".pid")
		file, err = os.Create(p.pidf)
		if err == nil {
			_, err = file.WriteString(strconv.Itoa(p.pid))
			if err == nil {
				file.Sync()
			}
			file.Close()
		}
	}
	return err
}

func (p *Poller) LogDebugInfo() {

	var err error
	var hostname string
	var st syscall.Sysinfo_t

	Log.Debug("Options: path=[%s], config=[%s], daemon=%v, debug=%v, loglevel=%d", 
		p.Args.Path, p.Args.Config, p.Args.Daemon, p.Args.Debug, p.Args.LogLevel)
	hostname, err  = os.Hostname()
	Log.Debug("Running on [%s]: system [%s], arch [%s], CPUs=%d", 
		hostname, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	Log.Debug("Poller Go build version [%s]", runtime.Version())
	
	st = syscall.Sysinfo_t{}
	err = syscall.Sysinfo(&st)
	if err == nil {
		Log.Debug("System uptime [%d], Memory [%d] / Free [%d]. Running processes [%d]", 
			st.Uptime, st.Totalram, st.Freeram, st.Procs)
	}
}


func ReadConfig(harvest_path, config_fn, name string) (*yaml.Node, *yaml.Node, error) {
	var err error
	var config, pollers, p, exporters, defaults *yaml.Node

	config, err = yaml.Import(path.Join(harvest_path, config_fn))

	if err == nil {

		pollers = config.GetChild("Pollers")
		defaults = config.GetChild("Defaults")

		if pollers == nil {
			err = errors.New("No pollers defined")
		} else {
			p = pollers.GetChild(name)
			if p == nil {
				err = errors.New("Poller [" + name + "] not defined")
			} else if defaults != nil {
				p.Union(defaults, false)
			}
		}
	}

	if err == nil && p != nil {

		exporters = config.GetChild("Exporters")
		if exporters == nil {
			Log.Warn("No exporters defined in config [%s]", config)
		} else {
			requested := p.PopChild("exporters")
			redundant := make([]*yaml.Node, 0)
			if requested != nil {
				for _, e := range exporters.Children {
					if !requested.HasInValues(e.Name) {
						redundant = append(redundant, e)
					}
				}
				for _, e := range redundant {
					exporters.PopChild(e.Name)
				}
			}
		}
	}

	return p, exporters, err
}



func main() {
    p := New()
    err := p.Init()

    if err != nil {
        panic(err)
    }

    p.Start()
}
