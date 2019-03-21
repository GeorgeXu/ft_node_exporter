package handler

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/node_exporter/kv"
)

type kvHandler struct {
	unfilteredHandler http.Handler
	// exporterMetricsRegistry is a separate registry for the metrics about
	// the exporter itself.
	exporterMetricsRegistry *prometheus.Registry
}

func NewKvHandler() *kvHandler {
	h := &kvHandler{
		exporterMetricsRegistry: prometheus.NewRegistry(),
	}

	if ih, err := h.innerHandler(); err != nil {
		log.Printf("[error] couldn't create metric handler: %s", err)
	} else {
		h.unfilteredHandler = ih
	}

	return h
}

func (h *kvHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filters := r.URL.Query()["collect[]"]
	if strings.Contains(r.URL.Path, "json") {
		kv.JsonFormat = true
	} else {
		kv.JsonFormat = false
	}

	// log.Printf("[debug] kv collect query:", filters)

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

func (h *kvHandler) innerHandler(f ...string) (http.Handler, error) {
	c, err := kv.NewKvCollector(f...)
	if err != nil {
		return nil, fmt.Errorf("couldn't create collector: %s", err)
	}

	if len(f) == 0 {
		collectors := []string{}
		for _c := range c.Collectors {
			collectors = append(collectors, _c)
		}

		sort.Strings(collectors)
		log.Printf("Enabled kv collectors(%d):", len(collectors))

		for _, _c := range collectors {
			log.Printf("[info]  - %s", _c)
		}
	}

	r := prometheus.NewRegistry()
	if err := r.Register(c); err != nil {
		return nil, fmt.Errorf("couldn't register kv collector: %s", err)
	}

	handler := promhttp.HandlerFor(
		prometheus.Gatherers{h.exporterMetricsRegistry, r},
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		},
	)

	return handler, nil
}
