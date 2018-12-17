package handler

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/node_exporter/osquery"
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
	f := r.URL.Query()["collect[]"]
	if len(f) == 0 {
		h.unfilteredHandler.ServeHTTP(w, r)
		return
	}

	fh, err := h.innerHandler(f...)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("couldn't create filtered metrics handler: %s", err)))
		return
	}

	fh.ServeHTTP(w, r)
}

func (h *envInfoHandler) innerHandler(f ...string) (http.Handler, error) {
	c, err := osquery.NewOSQueryCollector(f...)
	if err != nil {
		return nil, fmt.Errorf("couldn't create collector: %s", err)
	}

	if len(f) == 0 {
		collectors := []string{}
		for _c := range c.Collectors {
			collectors = append(collectors, _c)
		}
		sort.Strings(collectors)
		for _, _c := range collectors {
			log.Info(" - %s", _c)
		}
	}

	handler := promhttp.HandlerFor(
		prometheus.Gatherers{h.exporterMetricsRegistry},
		promhttp.HandlerOpts{
			ErrorLog:      log.NewErrorLogger(),
			ErrorHandling: promhttp.ContinueOnError,
		},
	)

	return handler, nil
}
