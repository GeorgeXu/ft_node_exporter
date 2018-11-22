package cloudcare

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/prometheus/config"
)

func loop(s *Storage) {

	sp := &scrape{
		storage: s,
	}

	ticker := time.NewTicker(time.Duration(thecfg.GlobalConfig.ScrapeInterval))
	defer ticker.Stop()

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

		contentType, err := sp.scrape(&buf)

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

func reloadConfig(filename string, logger log.Logger, rls ...func(*config.Config) error) (err error) {
	level.Info(logger).Log("msg", "Loading configuration file", "filename", filename)

	defer func() {
		if err == nil {
			//configSuccess.Set(1)
			//configSuccessTime.SetToCurrentTime()
		} else {
			//configSuccess.Set(0)
		}
	}()

	conf, err := config.LoadFile(filename)
	if err != nil {
		fmt.Printf("load err: %s", err)
		return fmt.Errorf("couldn't load configuration (--config.file=%q): %v", filename, err)
	}

	thecfg = conf

	failed := false
	for _, rl := range rls {
		if err := rl(conf); err != nil {
			level.Error(logger).Log("msg", "Failed to apply configuration", "err", err)
			failed = true
		}
	}
	if failed {
		return fmt.Errorf("one or more errors occurred while applying the new configuration (--config.file=%q)", filename)
	}
	level.Info(logger).Log("msg", "Completed loading of configuration file", "filename", filename)
	return nil
}

var remoteStorage *Storage
var thecfg *config.Config
var chStop chan struct{}

func Start() error {

	var l promlog.AllowedLevel
	l.Set("info")
	logger := promlog.New(l)

	chStop = make(chan struct{})

	RemoteFlushDeadline := time.Duration(60 * time.Second)

	remoteStorage = NewStorage(log.With(logger, "component", "remote"), nil, time.Duration(RemoteFlushDeadline))

	reloaders := []func(cfg *config.Config) error{
		remoteStorage.ApplyConfig,
	}

	err := reloadConfig("cfg.yml", logger, reloaders...)

	if err != nil {
		return err
	}

	loop(remoteStorage)

	return nil
}
