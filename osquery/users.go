package osquery

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type userCollector struct {
	entries *prometheus.Desc
}

func init() {
	registerCollector("arp", true, NewUserCollector)
}

func NewUserCollector() (Collector, error) {
	return &userCollector{
		entries: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "user", "entries"),
			"system users",
			[]string{"users"}, nil,
		),
	}, nil
}

func (c *userCollector) Update(ch chan<- prometheus.Metric) error {
	b64, err := doQuery("select * from users")

	if err != nil {
		return fmt.Errorf("osquery: could not get uses: %s", err)
	}

	ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, float64(0), b64)
	return nil
}
