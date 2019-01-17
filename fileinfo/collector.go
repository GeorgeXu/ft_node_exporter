package fileinfo

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

var (
	factories      = make(map[string]func(*fileInfoCfg) (Collector, error))
	collectorState = make(map[string]bool)
	factoryArgs    = make(map[string]*fileInfoCfg)
)

type Collector interface {
	Update(ch chan<- prometheus.Metric) error
}

type fileInfoHandler struct {
	unfilteredHandler http.Handler
}

func registerCollector(collector string, isDefaultEnabled bool, factory func(*fileInfoCfg) (Collector, error), arg *fileInfoCfg) {
	collectorState[collector] = isDefaultEnabled
	factories[collector] = factory
	if arg != nil {
		factoryArgs[collector] = arg
	}
}

type FileInfoCollector struct {
	Collectors map[string]Collector
}

func NewFileInfoCollector(filters ...string) (*FileInfoCollector, error) {
	f := make(map[string]bool)
	for _, filter := range filters {
		enabled, exist := collectorState[filter]
		if !exist {
			return nil, fmt.Errorf("missing collector: %s", filter)
		}
		if !enabled {
			return nil, fmt.Errorf("disabled collector: %s", filter)
		}
		f[filter] = true
	}

	collectors := make(map[string]Collector)
	for key, enabled := range collectorState {
		if enabled {
			collector, err := factories[key](factoryArgs[key]) // call NewxxxCollector()
			if err != nil {
				continue
			}
			if len(f) == 0 || f[key] {
				collectors[key] = collector
			}
		}
	}
	return &FileInfoCollector{Collectors: collectors}, nil
}

func (c FileInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	//ch <- scrapeDurationDesc
	//ch <- scrapeSuccessDesc

}

func (c FileInfoCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Collectors))

	log.Debugf("fileinfo try collect...")

	for name, _c := range c.Collectors {
		go func(name string, ec Collector) {
			execute(name, ec, ch)
			wg.Done()
		}(name, _c)
	}
	wg.Wait()
}

func execute(name string, c Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)

	if err != nil {
		log.Errorf("ERROR: %s collector failed after %fs: %s", name, duration.Seconds(), err)
	}
}
