package handler

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/node_exporter/envinfo"
)

type envInfoHandler struct {
	unfilteredHandler http.Handler
	// exporterMetricsRegistry is a separate registry for the metrics about
	// the exporter itself.
	exporterMetricsRegistry *prometheus.Registry
}

func NewEnvInfoHandler() *envInfoHandler {
	h := &envInfoHandler{
		exporterMetricsRegistry: prometheus.NewRegistry(),
	}

	if ih, err := h.innerHandler(); err != nil {
		log.Fatalf("couldn't create metric handler: %s", err)
	} else {
		h.unfilteredHandler = ih
	}

	return h
}

func (h *envInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filters := r.URL.Query()["collect[]"]

	log.Debugln("env collect query:", filters)

	if len(filters) == 0 {
		h.unfilteredHandler.ServeHTTP(w, r)
		return
	}

	fh, err := h.innerHandler(filters...)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("couldn't create filtered metrics handler: %s", err)))
		return
	}

	fh.ServeHTTP(w, r)
}

func (h *envInfoHandler) innerHandler(f ...string) (http.Handler, error) {
	c, err := envinfo.NewEnvInfoCollector(f...)
	if err != nil {
		return nil, fmt.Errorf("couldn't create collector: %s", err)
	}

	if len(f) == 0 {
		collectors := []string{}
		for _c := range c.Collectors {
			collectors = append(collectors, _c)
		}

		sort.Strings(collectors)
		log.Infof("Enabled env collectors(%d):", len(collectors))

		for _, _c := range collectors {
			log.Infof(" - %s", _c)
		}
	}

	r := prometheus.NewRegistry()
	if err := r.Register(c); err != nil {
		return nil, fmt.Errorf("couldn't register env_info collector: %s", err)
	}

	handler := promhttp.HandlerFor(
		prometheus.Gatherers{h.exporterMetricsRegistry, r},
		promhttp.HandlerOpts{
			ErrorLog:      log.NewErrorLogger(),
			ErrorHandling: promhttp.ContinueOnError,
		},
	)

	return handler, nil
}
