package cloudcare

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/node_exporter/cfg"
	"github.com/prometheus/node_exporter/rtpanic"
)

const (
	TagUploaderUID = `uploader_uid`
	TagHost        = `host`
	TagProbeName   = `probe_name`
)

var (
	HostName string
)

func init() {

	var err error
	HostName, err = os.Hostname()
	if err != nil {
		log.Printf("[error] %s, ignored", err.Error())
	}
}

func AddTags(s *model.Sample) {
	s.Metric[model.LabelName(TagUploaderUID)] = model.LabelValue(cfg.Cfg.UploaderUID)
}

func Start(remoteHost string, scrapehost string, interval int64) error {

	s, err := NewStorage(remoteHost, time.Duration(60*time.Second))
	if err != nil {
		return err
	}

	var f rtpanic.RecoverCallback
	f = func(_ []byte, _ error) {

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
				fmt.Println("[error] scrape error:", err)
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
