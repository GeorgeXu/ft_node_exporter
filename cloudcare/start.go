package cloudcare

import (
	"bytes"
	"log"
	"os"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/node_exporter/cfg"
	"github.com/prometheus/node_exporter/rtpanic"
	"github.com/prometheus/node_exporter/utils"
)

const (
	TagUploaderUID = `uploader_uid`
	TagHost        = `host`
	TagProbeName   = `probe_name`
)

var (
	HostName string
	sem      *utils.Sem // 退出信号量
)

func init() {

	var err error
	HostName, err = os.Hostname()
	if err != nil {
		log.Printf("[error] get host name faiel: %s", err.Error())
	}
}

func AddTags(s *model.Sample) {
	s.Metric[model.LabelName(TagUploaderUID)] = model.LabelValue(cfg.Cfg.UploaderUID)
}

func Start(remoteHost string, scrapehost string, interval int64) error {

	s, err := NewStorage(remoteHost, time.Duration(time.Duration(interval)*time.Millisecond))
	if err != nil {
		return err
	}

	var f rtpanic.RecoverCallback
	f = func(_ []byte, _ error) {

		time.Sleep(time.Second * 1)

		defer rtpanic.Recover(f, nil)

		sp := &scrape{
			storage: s,
		}

		ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
		defer ticker.Stop()

		for {
			var buf bytes.Buffer

			var (
				start = time.Now()
			)

			contentType, err := sp.scrape(&buf, scrapehost)

			if err != nil {
				log.Println("[error] scrape error:", err)
			} else {
				sp.appendScrape(buf.Bytes(), contentType, start)
			}

			select {
			case <-ticker.C:
			}
		}
	}

	go f(nil, nil)

	return nil
}
