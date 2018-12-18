package envinfo

import (
	"fmt"
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

type sshdCollector struct {
	entries *prometheus.Desc
}

var (
	sshdConfigures = []string{
		`/etc/ssh/sshd_config`,
	}
)

func init() {
	registerCollector("sshd", true, NewSSHDCollector)
}

func NewSSHDCollector() (Collector, error) {
	return &sshdCollector{
		entries: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "sshd", "configure"),
			"sshd configure",
			[]string{"raw"}, nil,
		),
	}, nil
}

func (c *sshdCollector) Update(ch chan<- prometheus.Metric) error {
	for _, f := range sshdConfigures {
		if _, err := os.Stat(f); err != nil { // file not exists
			continue
		}

		raw, err := doCat(f)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, float64(-1), raw)
		return nil
	}
	return fmt.Errorf("sshd configure not found")
}
