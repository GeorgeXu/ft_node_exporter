// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"sort"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/prometheus/node_exporter/cloudcare"
	"github.com/prometheus/node_exporter/collector"
	"github.com/prometheus/node_exporter/git"
	"gopkg.in/alecthomas/kingpin.v2"
)

// handler wraps an unfiltered http.Handler but uses a filtered handler,
// created on the fly, if filtering is requested. Create instances with
// newHandler.
type handler struct {
	unfilteredHandler http.Handler
	// exporterMetricsRegistry is a separate registry for the metrics about
	// the exporter itself.
	exporterMetricsRegistry *prometheus.Registry
	includeExporterMetrics  bool
}

func newHandler(includeExporterMetrics bool) *handler {
	h := &handler{
		exporterMetricsRegistry: prometheus.NewRegistry(),
		includeExporterMetrics:  includeExporterMetrics,
	}
	if h.includeExporterMetrics {
		h.exporterMetricsRegistry.MustRegister(
			prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
			prometheus.NewGoCollector(),
		)
	}
	if ih, err := h.innerHandler(); err != nil {
		log.Fatalf("Couldn't create metrics handler: %s", err)
	} else {
		h.unfilteredHandler = ih
	}
	return h
}

// ServeHTTP implements http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filters := r.URL.Query()["collect[]"]
	log.Debugln("collect query:", filters)

	if len(filters) == 0 {
		// No filters, use the prepared unfiltered handler.
		h.unfilteredHandler.ServeHTTP(w, r)
		return
	}
	// To serve filtered metrics, we create a filtering handler on the fly.
	filteredHandler, err := h.innerHandler(filters...)
	if err != nil {
		log.Warnln("Couldn't create filtered metrics handler:", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Couldn't create filtered metrics handler: %s", err)))
		return
	}
	filteredHandler.ServeHTTP(w, r)
}

// innerHandler is used to create buth the one unfiltered http.Handler to be
// wrapped by the outer handler and also the filtered handlers created on the
// fly. The former is accomplished by calling innerHandler without any arguments
// (in which case it will log all the collectors enabled via command-line
// flags).
func (h *handler) innerHandler(filters ...string) (http.Handler, error) {
	nc, err := collector.NewNodeCollector(filters...)
	if err != nil {
		return nil, fmt.Errorf("couldn't create collector: %s", err)
	}

	// Only log the creation of an unfiltered handler, which should happen
	// only once upon startup.
	if len(filters) == 0 {
		log.Infof("Enabled collectors:")
		collectors := []string{}
		for n := range nc.Collectors {
			collectors = append(collectors, n)
		}
		sort.Strings(collectors)
		for _, n := range collectors {
			log.Infof(" - %s", n)
		}
	}

	r := prometheus.NewRegistry()
	r.MustRegister(version.NewCollector("node_exporter"))
	if err := r.Register(nc); err != nil {
		return nil, fmt.Errorf("couldn't register node collector: %s", err)
	}
	handler := promhttp.HandlerFor(
		prometheus.Gatherers{h.exporterMetricsRegistry, r},
		promhttp.HandlerOpts{
			ErrorLog:      log.NewErrorLogger(),
			ErrorHandling: promhttp.ContinueOnError,
		},
	)
	if h.includeExporterMetrics {
		// Note that we have to use h.exporterMetricsRegistry here to
		// use the same promhttp metrics for all expositions.
		handler = promhttp.InstrumentMetricHandler(
			h.exporterMetricsRegistry, handler,
		)
	}
	return handler, nil
}

func main() {
	var (
		// listenAddress = kingpin.Flag(
		// 	"web.listen-address",
		// 	"Address on which to expose metrics and web interface.",
		// ).Default(":9100").String()
		metricsPath = kingpin.Flag(
			"web.telemetry-path",
			"Path under which to expose metrics.",
		).Default("/metrics").String()
		disableExporterMetrics = kingpin.Flag(
			"web.disable-exporter-metrics",
			"Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).",
		).Bool()

		singleton   = kingpin.Flag("singleton", "run as single").Bool()
		init        = kingpin.Flag("init", `init collector`).Bool()
		upgrade     = kingpin.Flag("upgrade", ``).Bool()
		host        = kingpin.Flag("host", `eg. ip addr`).String()
		remoteHost  = kingpin.Flag("remote-host", `data bridge addr`).String()
		scrapehost  = kingpin.Flag("scrapehost", `for test`).String()
		uniqueid    = kingpin.Flag("unique-id", ``).String()
		instanceid  = kingpin.Flag("instance-id", ``).String()
		ak          = kingpin.Flag("ak", ``).String()
		sk          = kingpin.Flag("sk", ``).String()
		port        = kingpin.Flag("port", `web listen port`).Default("9100").String()
		cfgFile     = kingpin.Flag("cfg", `configure file`).Default("cfg.yml").String()
		versionInfo = kingpin.Flag("ver", "show version info").Bool()
	)

	log.AddFlags(kingpin.CommandLine)
	// kingpin.Version(version.Print("node_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *versionInfo {
		fmt.Printf(`Version:        %s
Sha1:           %s
Build At:       %s
Golang Version: %s
`, git.Version, git.Sha1, git.BuildAt, git.Golang)
		return
	}

	if *init {
		args := &cloudcare.CCCmdArgs{
			Host:       *host,
			UniqueID:   *uniqueid,
			InstanceID: *instanceid,
			AK:         *ak,
			SK:         *sk,
			Port:       *port,
		}
		if err := cloudcare.InitHandler(args, *singleton); err != nil {
			panic(err)
		}

		return

	} else if *upgrade {

	}

	if err := cloudcare.ReloadConfig(*cfgFile); err != nil {
		panic(err)
	}

	if *singleton {
		var scu *url.URL
		var err error
		if *scrapehost != "" {
			scu, err = url.Parse(*scrapehost)
			if err != nil {
				panic(err)
			}
		}
		if err := cloudcare.Start(*remoteHost, *scrapehost); err != nil {
			panic(err)
		}
		if scu != nil {
			time.Sleep(60 * 60 * time.Second)
			return
		}
	}

	//log.Infoln("Starting node_exporter", version.Info())
	//log.Infoln("Build context", version.BuildContext())

	http.Handle(*metricsPath, newHandler(!*disableExporterMetrics))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Node Exporter</title></head>
			<body>
			<h1>Node Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	listenAddress := fmt.Sprintf("0.0.0.0:%s", cloudcare.GetListenPort())

	log.Infoln("Listening on", listenAddress)
	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		log.Fatal(err)
	}

}
