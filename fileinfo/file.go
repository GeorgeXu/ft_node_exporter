package fileinfo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/node_exporter/cfg"
	"github.com/prometheus/node_exporter/cloudcare"
)

const namespace = "file"

type fileInfoCfg struct {
	Configures map[string][]string `json:"configures"`
}

type fileCollector struct {
	cfg     *fileInfoCfg
	entries *prometheus.Desc
}

func NewFileCollector(cfg *fileInfoCfg) (Collector, error) {
	return &fileCollector{
		cfg: cfg,
		entries: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "info"),
			"", []string{"fileinfo", cloudcare.TagUploaderUID, cloudcare.TagHost}, nil)}, nil
}

func Init(cfgpath string) {
	var cfg fileInfoCfg
	j, err := ioutil.ReadFile(cfgpath)
	if err != nil {
		log.Fatalf("init file info failed: %s", cfgpath, err)
	}

	if err := json.Unmarshal(j, &cfg); err != nil {
		log.Fatalf("json load file info failed: %s", cfgpath, err)
	}

	registerCollector("fileinfo", true, NewFileCollector, &cfg)
}

func (ec *fileCollector) Update(ch chan<- prometheus.Metric) error {
	getFilesInfo(ec, ch)
	return nil
}

var totalSize int64

func getFilesInfo(ec *fileCollector, ch chan<- prometheus.Metric) error {

	var err error
	var buf bytes.Buffer

	totalSize = 0

	tw := tar.NewWriter(&buf)
	for name, files := range ec.cfg.Configures {
		updateTar(tw, &taritem{
			path: name,
			dir:  true,
		})
		for _, f := range files {
			finfo, err := os.Stat(f)
			if err != nil {
				continue
			}
			if finfo.IsDir() {
				addItemToTar(tw, f, true, name)
				addDirToTar(tw, f, name)
			} else {
				//TODO: check file size?
				totalSize += finfo.Size()
				addItemToTar(tw, f, false, name)
			}
		}
	}

	tw.Close()

	var gzbuf bytes.Buffer
	gzwr := gzip.NewWriter(&gzbuf)
	_, err = gzwr.Write(buf.Bytes())
	if err != nil {
		gzwr.Close()
		return err
	}

	if err = gzwr.Close(); err != nil {
		return err
	}

	raw := base64.RawURLEncoding.EncodeToString(gzbuf.Bytes())
	ch <- newEnvMetric(ec, raw)
	return nil
}

func newEnvMetric(ec *fileCollector, envVal string) prometheus.Metric {
	return prometheus.MustNewConstMetric(ec.entries, prometheus.GaugeValue, float64(-1), envVal,
		// 此处追加两个 tag, 在 queue-manager 那边也会追加, 有重复, 待去掉
		cfg.Cfg.UploaderUID, cfg.Cfg.Host)
}

type taritem struct {
	path    string
	dir     bool
	content []byte
}

func updateTar(tw *tar.Writer, item *taritem) error {
	h := &tar.Header{
		Name: item.path,
	}
	if item.dir {
		h.Typeflag = tar.TypeDir
		h.Mode = 0777
	} else {
		h.Mode = 0666
	}
	var err error
	if err = tw.WriteHeader(h); err != nil {
		return err
	}

	if !item.dir {
		_, err = tw.Write(item.content)
	}

	return err
}

func addItemToTar(tw *tar.Writer, path string, dir bool, namespace string) {
	var content []byte
	var err error
	if !dir {
		content, err = ioutil.ReadFile(path)
		if err != nil {
			return
		}
		totalSize += int64(len(content))
	}

	tarpath := namespace
	if !strings.HasPrefix(path, string(os.PathSeparator)) {
		tarpath += string(os.PathSeparator)
	}
	tarpath += path
	item := &taritem{
		path:    tarpath,
		dir:     dir,
		content: content,
	}
	updateTar(tw, item)
}

func addDirToTar(tw *tar.Writer, path string, namespace string) {

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return
	}
	for _, f := range files {
		fullpath := filepath.Join(path, f.Name())
		if f.IsDir() {
			addItemToTar(tw, fullpath, true, namespace)
			addDirToTar(tw, fullpath, namespace)
		} else {
			addItemToTar(tw, fullpath, false, namespace)
		}
	}
}
