package envinfo

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type userCollector struct {
	entries *prometheus.Desc
}

func init() {
	registerCollector("users", true, NewUserCollector)
}

func NewUserCollector() (Collector, error) {
	return &userCollector{
		entries: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "user", "list"),
			"system users",
			[]string{"json"}, nil,
		),
	}, nil
}

func (c *userCollector) Update(ch chan<- prometheus.Metric) error {
	j, err := doQuery("select * from users")

	if err != nil {
		return fmt.Errorf("osquery: could not get uses: %s", err)
	}

	ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, float64(-1), j)
	return nil
}
