package envinfo

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/node_exporter/cfg"
	"github.com/prometheus/node_exporter/cloudcare"
)

const (
	envCollectorTypeCat     = `cat`
	envCollectorTypeOSQuery = `osquery`

	envPlatformWindows = `windows`
	envPlatformLinux   = `linux`

	fileSep = "\r\n"
)

type envCfg struct {
	Platform  string `json:"platform"`
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
	desc *prometheus.Desc
	cfg  *envCfg
}

var (
	hostKey      = "ft_" + cloudcare.TagHost
	uploaduidKey = "ft_" + cloudcare.TagUploaderUID
)

func NewEnvCollector(cfg *envCfg) (Collector, error) {
	c := &envCollector{
		cfg: cfg,
	}
	if cfg.Type == envCollectorTypeCat {
		cfg.Tags = append(cfg.Tags, uploaduidKey, hostKey)
		c.desc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", cfg.SubSystem),
			cfg.Help, cfg.Tags, nil)

	}
	return c, nil
}

func Init(cfgFile string) {
	var envCfgs envCfgs
	j, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatalf("[fatal] open %s failed: %s", cfgFile, err)
	}

	if err := json.Unmarshal(j, &envCfgs); err != nil {
		log.Fatalf("[fatal] yaml load %s failed: %s", cfgFile, err)
	}

	for _, ec := range envCfgs.Envs {
		if ec.Platform != "" && ec.Platform == runtime.GOOS {

			//ec.Tags = append(ec.Tags, cloudcare.TagUploaderUID, cloudcare.TagHost) // 追加默认 tags
			registerCollector(ec.SubSystem, ec.Enabled, NewEnvCollector, ec)

		} else {
			log.Printf("[info] skip collector %s(platform: %s)", ec.SubSystem, ec.Platform)
		}
	}

}

func (ec *envCollector) Update(ch chan<- prometheus.Metric) error {
	switch ec.cfg.Type {
	case envCollectorTypeCat:
		return ec.catUpdate(ch)
	case envCollectorTypeOSQuery:
		return ec.osqueryUpdate(ch)
	default:
		log.Printf("[warn] unsupported env collector type: %s", ec.cfg.Type)
		return nil
	}
	return nil
}

func (ec *envCollector) catUpdate(ch chan<- prometheus.Metric) error {
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
	return prometheus.MustNewConstMetric(ec.desc, prometheus.GaugeValue, float64(-1), envVal,
		// 此处追加两个 tag, 在 queue-manager 那边也会追加, 有重复, 待去掉
		cfg.Cfg.UploaderUID, cfg.Cfg.Host)
	return nil
}

func (ec *envCollector) osqueryUpdate(ch chan<- prometheus.Metric) error {
	res, err := doQuery(ec.cfg.SQL)
	if err != nil {
		return err
	}
	_ = res

	n := len(res)
	if n == 0 {
		return nil
	}

	entry := res[0]
	var keys = []string{hostKey, uploaduidKey}
	for k := range entry {
		keys = append(keys, k)
	}

	desc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", ec.cfg.SubSystem),
		ec.cfg.Help,
		keys,
		nil,
	)

	for _, m := range res {
		m[uploaduidKey] = cfg.Cfg.UploaderUID
		m[hostKey] = cfg.Cfg.Host
		var vals []string
		for _, k := range keys {
			vals = append(vals, m[k])
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, -1, vals...)
	}

	return nil
}
