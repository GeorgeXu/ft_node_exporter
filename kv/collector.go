package kv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "kv_node"

type queryResult struct {
	rawJson    string
	formatJson []map[string]string
}

var (
	OSQuerydPath = ""
	// run osquery:  ./osqueryd -S --json 'select * from users'
	//   -S: run as shell mode
	//   --json: output result in json format

	factories      = make(map[string]func(*kvCfg) (Collector, error))
	collectorState = make(map[string]bool)
	factoryArgs    = make(map[string]*kvCfg)

	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"envinfo: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"envinfo: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

type Collector interface {
	Update(ch chan<- prometheus.Metric) error
}

func registerCollector(collector string, isDefaultEnabled bool, factory func(*kvCfg) (Collector, error), arg *kvCfg) {
	collectorState[collector] = isDefaultEnabled
	factories[collector] = factory
	if arg != nil {
		factoryArgs[collector] = arg
	}
}

type KvCollector struct {
	Collectors map[string]Collector
}

func NewKvCollector(filters ...string) (*KvCollector, error) {
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
	return &KvCollector{Collectors: collectors}, nil
}

func (c KvCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

func (c KvCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Collectors))

	// log.Printf("[debug] envinfo try collect...")

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
		log.Printf("[error] collector %s failed after %fs: %s", name, duration.Seconds(), err)
	}
}

func doCat(path string) (string, error) {

	cmd := exec.Command(`cat`, []string{path}...)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	res := base64.RawURLEncoding.EncodeToString(out)

	return res, nil
}

func doQuery(sql string) (*queryResult, error) {
	cmd := exec.Command(OSQuerydPath, []string{`-S`, `--json`, sql}...)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var res queryResult
	if !JsonFormat {
		err = json.Unmarshal(out, &res.formatJson)
		if err != nil {
			return nil, err
		}
	} else {
		res.rawJson = base64.RawURLEncoding.EncodeToString(out)
	}

	return &res, nil
}
