package handler

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/node_exporter/fileinfo"
)

type fileInfoHandler struct {
	unfilteredHandler http.Handler
}

func NewFileInfoHandler() *fileInfoHandler {
	h := &fileInfoHandler{}

	if ih, err := h.innerHandler(); err != nil {
		log.Printf("[error] couldn't create fileinfo handler: %s", err)
	} else {
		h.unfilteredHandler = ih
	}

	return h
}

func (h *fileInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filters := r.URL.Query()["collect[]"]

	//log.Printf("[debug] env collect query:", filters)

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

func (h *fileInfoHandler) innerHandler(f ...string) (http.Handler, error) {
	c, err := fileinfo.NewFileInfoCollector(f...)
	if err != nil {
		return nil, fmt.Errorf("couldn't create collector: %s", err)
	}

	// if len(f) == 0 {
	// 	collectors := []string{}
	// 	for _c := range c.Collectors {
	// 		collectors = append(collectors, _c)
	// 	}

	// 	sort.Strings(collectors)
	// 	log.Printf("Enabled file collectors(%d):", len(collectors))

	// 	for _, _c := range collectors {
	// 		log.Printf(" - %s", _c)
	// 	}
	// }

	r := prometheus.NewRegistry()
	if err := r.Register(c); err != nil {
		return nil, fmt.Errorf("couldn't register file_info collector: %s", err)
	}

	handler := promhttp.HandlerFor(
		prometheus.Gatherers{r},
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		},
	)

	return handler, nil
}
