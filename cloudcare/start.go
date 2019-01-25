package cloudcare

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promlog"
)

const (
	TagUploaderUID = `uploader_uid`
	TagHost        = `host`
)

var (
	CorsairUploaderUID    string
	CorsairTeamID         string
	CorsairSK             string
	CorsairAK             string
	CorsairHost           string = `default`
	CorsairPort           int
	CorsairScrapeInterval int
)

func AddTags(s *model.Sample) {
	s.Metric[model.LabelName(`uploader_uid`)] = model.LabelValue(CorsairUploaderUID)
	s.Metric[model.LabelName(`host`)] = model.LabelValue(CorsairHost)
}

func loop(s *Storage, scrapeurl string, interval int) {

	sp := &scrape{
		storage: s,
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	var chStop chan struct{}

	for {
		var buf bytes.Buffer

		select {
		case <-chStop:
			return
		default:
		}

		var (
			start = time.Now()
		)

		contentType, err := sp.scrape(&buf, scrapeurl)

		if err == nil {
			sp.appendScrape(buf.Bytes(), contentType, start)
		} else {
			fmt.Println("scrape error:", err)
			return
		}

		select {
		case <-ticker.C:
		case <-chStop:
			return
		}
	}
}

func Start(remoteHost string, scrapehost string, interval int) error {

	var l promlog.AllowedLevel
	l.Set("info")
	logger := promlog.New(l)

	RemoteFlushDeadline := time.Duration(60 * time.Second)

	s := NewStorage(log.With(logger, "component", "remote"), nil, time.Duration(RemoteFlushDeadline))

	if err := s.applyConfig(remoteHost); err != nil {
		return err
	}

	go func() {
		time.Sleep(1 * time.Second)
		loop(s, scrapehost, interval)
	}()

	return nil
}
