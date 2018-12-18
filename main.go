// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"path"
	"time"

	"github.com/prometheus/common/log"
	"github.com/prometheus/node_exporter/cfg"
	"github.com/prometheus/node_exporter/cloudcare"
	"github.com/prometheus/node_exporter/collector"
	"github.com/prometheus/node_exporter/envinfo"
	"github.com/prometheus/node_exporter/git"
	"github.com/prometheus/node_exporter/handler"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	metricsPath            = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	envInfoPath            = kingpin.Flag("web.telemetry-env-info-path", "Path under which to expose env info.").Default("/env_infos").String()
	disableExporterMetrics = kingpin.Flag("web.disable-exporter-metrics", "Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).").Bool()

	flagSingleMode          = kingpin.Flag("single-mode", "run as single node").Default("0").Int()
	flagInit                = kingpin.Flag("init", `init collector`).Bool()
	flagUpgrade             = kingpin.Flag("upgrade", ``).Bool()
	flagHost                = kingpin.Flag("host", `eg. ip addr`).String()
	flagRemoteHost          = kingpin.Flag("remote-host", `data bridge addr`).Default("http://kodo.cloudcare.com/v1/write").String()
	flagScrapeInterval      = kingpin.Flag("scrape-interval", "frequency to upload data").Default("15").Int()
	flagTeamID              = kingpin.Flag("team-id", "User ID").String()
	flagCloudAssetID        = kingpin.Flag("cloud-asset-id", "cloud instance ID").String()
	flagAK                  = kingpin.Flag("ak", `Access Key`).String()
	flagSK                  = kingpin.Flag("sk", `Secret Key`).String()
	flagPort                = kingpin.Flag("port", `web listen port`).Default("9100").Int()
	flagCfgFile             = kingpin.Flag("cfg", `configure file`).Default("cfg.yml").String()
	flagVersionInfo         = kingpin.Flag("version", "show version info").Bool()
	flagEnableAllCollectors = kingpin.Flag("enable-all", "enable all collectors").Default("0").Int()
	flagInstallDir          = kingpin.Flag("install-dir", "install directory").Default("/usr/local/cloudcare").String()
)

func initCfg() error {
	cfg.Cfg.SingleMode = *flagSingleMode

	if *flagHost != "" {
		cfg.Cfg.Host = *flagHost
	}

	cfg.Cfg.RemoteHost = *flagRemoteHost
	cfg.Cfg.ScrapeInterval = *flagScrapeInterval
	cfg.Cfg.EnableAll = *flagEnableAllCollectors

	// unique-id 为必填参数
	if *flagTeamID == "" {
		log.Fatal("invalid unique-id")
	} else {
		cfg.Cfg.TeamID = *flagTeamID
	}

	if *flagCloudAssetID == "" {
		log.Fatal("invalid cloud assert id")
	} else {
		cfg.Cfg.CloudAssetID = *flagCloudAssetID
	}

	if *flagAK == "" {
		log.Fatal("invalid ak")
	} else {
		cfg.Cfg.AK = *flagAK
	}

	if *flagSK == "" {
		log.Fatal("invalid sk")
	} else {
		cfg.Cfg.SK = cfg.XorEncode(*flagSK)
	}

	cfg.Cfg.Port = *flagPort

	cfg.Cfg.Collectors = collector.ListAllCollectors()

	if cfg.Cfg.EnableAll == 1 {
		for k, _ := range cfg.Cfg.Collectors {
			cfg.Cfg.Collectors[k] = true
		}
	}

	return cfg.DumpConfig(*flagCfgFile)
}

func main() {

	log.AddFlags(kingpin.CommandLine)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *flagVersionInfo {
		fmt.Printf(`Version:        %s
Sha1:           %s
Build At:       %s
Golang Version: %s
`, git.Version, git.Sha1, git.BuildAt, git.Golang)
		return
	}

	if *flagInit {
		_ = initCfg()
		return
	} else if *flagUpgrade {
		// TODO
		return
	}

	cfg.LoadConfig(*flagCfgFile)
	cfg.DumpConfig(*flagCfgFile) // load 过程中可能会修改 cfg.Cfg, 需重新写入

	if cfg.Cfg.SingleMode == 1 {
		var scu *url.URL

		if err := cloudcare.Start(cfg.Cfg.RemoteHost, ""); err != nil {
			panic(err)
		}
		if scu != nil {
			time.Sleep(60 * 60 * time.Second)
			return
		}
	}

	envinfo.OSQuerydPath = path.Join(*flagInstallDir, `osqueryd`)

	http.Handle(*envInfoPath, handler.NewEnvInfoHandler())
	http.Handle(*metricsPath, handler.NewMetricHandler(!*disableExporterMetrics))

	listenAddress := fmt.Sprintf("0.0.0.0:%d", cfg.Cfg.Port)

	log.Infoln("Listening on", listenAddress)
	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		log.Fatal(err)
	}
}
