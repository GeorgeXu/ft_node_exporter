package kv

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

var forbidTags = []string{
	"alertname",
	"exported_",
	"__name__",
	"__scheme__",
	"__address__",
	"__metrics_path__",
	"__",
	"__meta_",
	"__tmp_",
	"__param_",
	"job",
	"instance",
	"le",
	"quantile",
}

const (
	kvCollectorTypeCat     = `cat`
	kvCollectorTypeOSQuery = `osquery`

	kvPlatformWindows = `windows`
	kvPlatformLinux   = `linux`

	fileSep = "\r\n"
)

type kvCfg struct {
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

type kvCfgs struct {
	Kvs []*kvCfg `json:"kvs"`
}

type kvCollector struct {
	desc *prometheus.Desc
	cfg  *kvCfg
}

var (
	JsonFormat = false
)

func NewNodeCollector(conf *kvCfg) (Collector, error) {
	c := &kvCollector{
		cfg: conf,
	}

	conf.Tags = append(conf.Tags, cloudcare.TagUploaderUID)
	c.desc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", conf.SubSystem),
		conf.Help, conf.Tags, nil)

	return c, nil
}

func Init(cfgFile string) {
	var kvCfgs kvCfgs
	j, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatalf("[fatal] open %s failed: %s", cfgFile, err)
	}

	if err := json.Unmarshal(j, &kvCfgs); err != nil {
		log.Fatalf("[fatal] yaml load %s failed: %s", cfgFile, err)
	}

	for _, kc := range kvCfgs.Kvs {
		if kc.Platform != "" && kc.Platform == runtime.GOOS {

			//kc.Tags = append(kc.Tags, cloudcare.TagUploaderUID, cloudcare.TagHost) // 追加默认 tags
			registerCollector(kc.SubSystem, kc.Enabled, NewNodeCollector, kc)

		} else {
			log.Printf("[info] skip collector %s(platform: %s)", kc.SubSystem, kc.Platform)
		}
	}

}

func (kc *kvCollector) Update(ch chan<- prometheus.Metric) error {
	switch kc.cfg.Type {
	case kvCollectorTypeCat:
		return kc.catUpdate(ch)
	case kvCollectorTypeOSQuery:
		return kc.osqueryUpdate(ch)
	default:
		log.Printf("[warn] unsupported env collector type: %s", kc.cfg.Type)
		return nil
	}
	return nil
}

func (kc *kvCollector) catUpdate(ch chan<- prometheus.Metric) error {
	var rawFileContents []string
	for _, f := range kc.cfg.Files {
		if _, err := os.Stat(f); err != nil {
			continue
		}

		raw, err := doCat(f)
		if err != nil {
			return err
		}
		rawFileContents = append(rawFileContents, raw)

		if kc.cfg.Any {
			break
		}
	}

	if len(rawFileContents) == 0 {
		log.Printf("[warn] no file read for %s, ignored", kc.cfg.SubSystem)
		return nil
	}

	raw := strings.Join(rawFileContents, fileSep)
	ch <- newEnvMetric(kc, raw)
	return nil
}

func newEnvMetric(kc *kvCollector, envVal string) prometheus.Metric {
	return prometheus.MustNewConstMetric(kc.desc, prometheus.GaugeValue, float64(-1), envVal,
		// 此处追加两个 tag, 在 queue-manager 那边也会追加, 有重复, 待去掉
		cfg.Cfg.UploaderUID)
	return nil
}

func (kc *kvCollector) osqueryUpdate(ch chan<- prometheus.Metric) error {
	res, err := doQuery(kc.cfg.SQL)
	if err != nil {
		return err
	}

	//集群模式下，兼容promtheous
	if !JsonFormat {
		n := len(res.formatJson)
		if n == 0 {
			return nil
		}

		uploaduidKey := "ft_" + cloudcare.TagUploaderUID //集群模式下 避免和osquery产生的结果冲突

		entry := res.formatJson[0]
		var keys = []string{uploaduidKey}
		var tuned []string
		for k := range entry {
			bforbid := false
			for _, ft := range forbidTags {
				if ft == k {
					bforbid = true
					break
				}
			}
			if bforbid {
				k = k + "_"
				tuned = append(tuned, k)
			}
			keys = append(keys, k)
		}

		desc := prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", kc.cfg.SubSystem),
			kc.cfg.Help,
			keys,
			nil,
		)

		for _, m := range res.formatJson {
			m[uploaduidKey] = cfg.Cfg.UploaderUID
			var vals []string
			for _, k := range keys {
				for _, tk := range tuned {
					if tk == k {
						k = k[:len(k)-1]
						break
					}
				}
				vals = append(vals, m[k])
			}
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, -1, vals...)
		}
	} else {
		ch <- newEnvMetric(kc, res.rawJson)
	}

	return nil
}
