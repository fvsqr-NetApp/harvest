package main

import (
    "strconv"
    "os/exec"
    "goharvest2/pkg/matrix"
    "runtime"
    "strings"
)

// used for selfmonitoring
var upCollectors, upExporters int

// selfMonitor updates the current status of the poller and writes to log.
// This includes: status of the target system, status and metadata of
// collectors. To store this data we use two Matrices: p.status
// and p.metadata.
func (p *Poller) SelfMonitor() (*matrix.Matrix, error) {

    var (
        ping float32
        ok bool
        tmpUpCollectors, tmpUpExporters int
    )

    // flush metadata
    p.status.Reset()
    p.metadata.Reset()

    // ping target system
    if ping, ok = p.ping(); ok {
        p.status.LazySetValueUint8("status", "host", 0)
        p.status.LazySetValueFloat32("ping", "host", ping)
    } else {
        p.status.LazySetValueUint8("status", "host", 1)
    }

    // add number of goroutines to metadata
    // @TODO: cleanup, does not belong to "status"
    p.status.LazySetValueInt("goroutines", "host", runtime.NumGoroutine())

    // update status of collectors
    for _, c := range p.collectors {
        code, status, msg := c.GetStatus()
        logger.Debug().Msgf("collector (%s:%s) status: (%d - %s) %s", c.GetName(), c.GetObject(), code, status, msg)

        if code == 0 {
            tmpUpCollectors++
        }

        key := c.GetName() + "." + c.GetObject()

        p.metadata.LazySetValueUint64("count", key, c.GetCollectCount())
        p.metadata.LazySetValueUint8("status", key, code)

        if msg != "" {
            if instance := p.metadata.GetInstance(key); instance != nil {
                instance.SetLabel("reason", msg)
            }
        }
    }

    // update status of exporters
    for _, e := range p.exporters {
        code, status, msg := e.GetStatus()
        logger.Debug().Msgf("exporter (%s) status: (%d - %s) %s", e.GetName(), code, status, msg)

        if code == 0 {
            tmpUpExporters++
        }

        key := e.GetClass() + "." + e.GetName()

        p.metadata.LazySetValueUint64("count", key, e.GetExportCount())
        p.metadata.LazySetValueUint8("status", key, code)

        if msg != "" {
            if instance := p.metadata.GetInstance(key); instance != nil {
                instance.SetLabel("reason", msg)
            }
        }
    }

    // @TODO if there are no "master" exporters, don't collect metadata
    for _, e := range p.exporters {
        if err := e.Export(p.metadata); err != nil {
            logger.Error().Stack().Err(err).Msg("export component metadata:")
        }
        if err := e.Export(p.status); err != nil {
            logger.Error().Stack().Err(err).Msg("export target metadata:")
        }
    }

    // only log when numbers have changes, since hopefully that happens rarely
    if tmpUpCollectors != upCollectors || tmpUpExporters != upExporters {
        logger.Info().Msgf("updated status, up collectors: %d (of %d), up exporters: %d (of %d)", tmpUpCollectors, len(p.collectors), tmpUpExporters, len(p.exporters))
    }
    upCollectors = tmpUpCollectors
    upExporters = tmpUpExporters

    return nil, nil
}

// ping target system, report if it's available or not
// and if available, response time
func (p *Poller) ping() (float32, bool) {

	cmd := exec.Command("ping", p.target, "-w", "5", "-c", "1", "-q")

	if out, err := cmd.Output(); err == nil {
		if x := strings.Split(string(out), "mdev = "); len(x) > 1 {
			if y := strings.Split(x[len(x)-1], "/"); len(y) > 1 {
				if p, err := strconv.ParseFloat(y[0], 32); err == nil {
					return float32(p), true
				}
			}
		}
	}
	return 0, false
}
