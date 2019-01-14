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

// Package collector includes all individual collectors to gather and export system metrics.
package collector

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// Namespace defines the common namespace to be used by all metrics.
const namespace = "node"

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"node_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"node_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

const (
	defaultEnabled  = true
	defaultDisabled = false
)

var (
	factories        = make(map[string]func() (Collector, error))
	collectorState   = make(map[string]bool)
	failedCollectors = make(map[string]int) // 失败次数过多, 则踢出收集器列表
)

func ListAllCollectors() map[string]bool {
	return collectorState
}

func SetCollector(collector string, isEnabled bool) {
	if _, ok := collectorState[collector]; !ok {
		// do nothing
		return
	}

	collectorState[collector] = isEnabled
}

func registerCollector(collector string, isDefaultEnabled bool, factory func() (Collector, error)) {

	collectorState[collector] = isDefaultEnabled

	factories[collector] = factory
}

// NodeCollector implements the prometheus.Collector interface.
type NodeCollector struct {
	Collectors map[string]Collector
}

// NewNodeCollector creates a new NodeCollector.
func NewNodeCollector(filters ...string) (*NodeCollector, error) {
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
			collector, err := factories[key]() // call NewxxxCollector()
			if err != nil {
				continue
				//return nil, err
			}
			if len(f) == 0 || f[key] {
				collectors[key] = collector
			}
		}
	}
	return &NodeCollector{Collectors: collectors}, nil
}

// Describe implements the prometheus.Collector interface.
func (n NodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (n NodeCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(n.Collectors))

	chFailed := make(chan string, len(n.Collectors))
	defer close(chFailed)

	log.Debugf("node-exporter try collect...")

	for name, c := range n.Collectors {
		go func(name string, c Collector) {
			execute(name, c, ch, chFailed)
			wg.Done()
		}(name, c)
	}
	wg.Wait() // 等待所有 collector 跑完

	for {
		select {
		case name := <-chFailed:
			n.addFailed(name)
		default:
			return
		}
	}
}

func (n NodeCollector) addFailed(name string) {
	if _, ok := failedCollectors[name]; !ok {
		failedCollectors[name] = 1
	} else {
		failedCollectors[name]++
	}

	// 由于默认开启了所有的收集器, 所以, 有一些收集器会不成功, 失败次数过多, 则将其踢出去
	if failedCollectors[name] > 3 {
		log.Warnf("WARN: %s collector failed %d times, remove it.", name, failedCollectors[name])
		delete(n.Collectors, name)
	} else {
		log.Warnf("WARN: %s collector failed %d times", name, failedCollectors[name])
	}
}

func execute(name string, c Collector, ch chan<- prometheus.Metric, chFailed chan<- string) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)

	if err != nil {
		log.Errorf("ERROR: %s collector failed after %fs: %s", name, duration.Seconds(), err)
		chFailed <- name
	} else {
		log.Debugf("OK: %s collector succeeded after %fs.", name, duration.Seconds())
	}
}

// Collector is the interface a collector has to implement.
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Update(ch chan<- prometheus.Metric) error
}

type typedDesc struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
}

func (d *typedDesc) mustNewConstMetric(value float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.desc, d.valueType, value, labels...)
}
