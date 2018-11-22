package cloudcare

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/textparse"
	"github.com/prometheus/prometheus/pkg/timestamp"
)

type scrape struct {
	storage *Storage
}

const acceptHeader = `application/openmetrics-text; version=0.0.1,text/plain;version=0.0.4;q=0.5,*/*;q=0.1`

var userAgentHeader = fmt.Sprintf("Prometheus/%s", version.Version)

func (s *scrape) scrape(w io.Writer) (string, error) {
	url := "http://0.0.0.0:9100/metrics"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Accept", acceptHeader)
	req.Header.Add("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", userAgentHeader)
	//req.Header.Set("X-Prometheus-Scrape-Timeout-Seconds", fmt.Sprintf("%f", s.timeout.Seconds()))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned HTTP status %s", resp.Status)

	}

	var gziper *gzip.Reader
	var buf *bufio.Reader

	if resp.Header.Get("Content-Encoding") != "gzip" {
		_, err = io.Copy(w, resp.Body)
		return "", err
	}

	if gziper == nil {

		buf = bufio.NewReader(resp.Body)
		gziper, err = gzip.NewReader(buf)
		if err != nil {
			return "", err
		}

	} else {
		buf.Reset(resp.Body)
		if err := gziper.Reset(buf); err != nil {
			return "", err
		}
	}

	_, err = io.Copy(w, gziper)
	gziper.Close()

	if err != nil {
		return "", err
	}

	return resp.Header.Get("Content-Type"), nil

}

func (s *scrape) appendScrape(b []byte, contentType string, ts time.Time) {
	var (
		p       = textparse.New(b, contentType)
		defTime = timestamp.FromTime(ts)
		err     error
	)

	for {
		var et textparse.Entry
		if et, err = p.Next(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		switch et {
		case textparse.EntryType:
			//sl.cache.setType(p.Type())
			//metricName, mType := p.Type()
			//fmt.Println(string(metricName), mType)
			continue
		case textparse.EntryHelp:
			//sl.cache.setHelp(p.Help())
			//metricName, mHelp := p.Help()
			//fmt.Println(string(metricName), string(mHelp))
			continue
		case textparse.EntryUnit:
			//sl.cache.setUnit(p.Unit())
			continue
		case textparse.EntryComment:
			continue
		default:
		}
		//total++

		t := defTime
		_, tp, v := p.Series()
		if tp != nil {
			t = *tp
		}

		//fmt.Println("series:", string(met))
		//fmt.Println("value:", v)

		var lset labels.Labels

		_ = p.Metric(&lset)
		//_ = mets
		//fmt.Println("mets:", mets)

		if lset == nil {
			continue
		}
		//fmt.Println(lset.String())

		s.storage.Add(lset, t, v)
	}

}
