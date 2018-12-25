package envinfo

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/node_exporter/cloudcare"
)

const (
	envCollectorTypeCat     = `cat`
	envCollectorTypeOSQuery = `osquery`

	fileSep = "\r\n"
)

type envCfg struct {
	Name      string `json:"name"`
	SubSystem string `json:"sub_system"`
	Type      string `json:"type"`
	SQL       string `json:"sql"`

	Files []string `json:"files"`
	Any   bool     `json:"any"`

	Tags    []string `json:"tags"`
	Help    string   `json:"help"`
	Enabled bool     `json:"enabled"`
}

type envCfgs struct {
	Envs []*envCfg `json:"envs"`
}

type envCollector struct {
	entries *prometheus.Desc
	cfg     *envCfg
}

func NewEnvCollector(cfg *envCfg) (Collector, error) {
	return &envCollector{
		cfg: cfg,
		entries: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, cfg.SubSystem, cfg.Name),
			cfg.Help, cfg.Tags, nil)}, nil
}

func Init(cfgFile string) {
	var envCfgs envCfgs
	j, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatal(err)
	}

	if err := json.Unmarshal(j, &envCfgs); err != nil {
		log.Fatal(err)
	}

	for _, ec := range envCfgs.Envs {
		ec.Tags = append(ec.Tags, cloudcare.TagCloudAssetID, cloudcare.TagHost) // 追加默认 tags
		registerCollector(ec.SubSystem, ec.Enabled, NewEnvCollector, ec)
	}
}

func (ec *envCollector) Update(ch chan<- prometheus.Metric) error {
	switch ec.cfg.Type {
	case envCollectorTypeCat:
		return catUpdate(ec, ch)
	case envCollectorTypeOSQuery:
		return osqueryUpdate(ec, ch)
	default:
		log.Printf("[warn] unsupported env collector type: %s", ec.cfg.Type)
		return nil
	}
	return nil
}

func catUpdate(ec *envCollector, ch chan<- prometheus.Metric) error {
	var rawFileContents []string
	for _, f := range ec.cfg.Files {
		if _, err := os.Stat(f); err != nil {
			continue
		}

		raw, err := doCat(f)
		if err != nil {
			return err
		}
		rawFileContents = append(rawFileContents, raw)

		if ec.cfg.Any {
			break
		}
	}

	if len(rawFileContents) == 0 {
		log.Printf("[warn] no file read for %s, ignored", ec.cfg.SubSystem)
		return nil
	}

	raw := strings.Join(rawFileContents, fileSep)
	ch <- newEnvMetric(ec, raw)
	return nil
}

func newEnvMetric(ec *envCollector, envVal string) prometheus.Metric {
	return prometheus.MustNewConstMetric(ec.entries, prometheus.GaugeValue, float64(-1), envVal,
		// 此处追加两个 tag, 在 queue-manager 那边也会追加, 有重复, 待去掉
		cloudcare.CorsairCloudAssetID, cloudcare.CorsairHost)
}

func osqueryUpdate(ec *envCollector, ch chan<- prometheus.Metric) error {
	j, err := doQuery(ec.cfg.SQL)
	if err != nil {
		return err
	}

	ch <- newEnvMetric(ec, j)
	return nil
}
