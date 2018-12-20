package envinfo

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type userCollector struct {
	entries *prometheus.Desc
}

func init() {
	// registerCollector("users", true, NewUserCollector, nil)
}

func NewUserCollector(_ *envCfg) (Collector, error) {

	var (
		subSystem = `user`
		name      = `list`
		help      = `system users`
		tags      = []string{`json`}
	)

	return &userCollector{
		entries: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subSystem, name),
			help, tags, nil)}, nil
}

func (c *userCollector) Update(ch chan<- prometheus.Metric) error {
	j, err := doQuery("select * from users")

	if err != nil {
		return fmt.Errorf("osquery: could not get uses: %s", err)
	}

	ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, float64(-1), j)
	return nil
}
