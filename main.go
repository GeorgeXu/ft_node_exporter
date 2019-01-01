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
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"path"

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
	metricsPath = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	envInfoPath = kingpin.Flag("web.telemetry-env-info-path", "Path under which to expose env info.").Default("/env_infos").String()

	metaPath = kingpin.Flag("web.meta-path", "Path under which to expose meta info.").Default("/meta").String()

	disableExporterMetrics = kingpin.Flag("web.disable-exporter-metrics", "Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).").Bool()

	flagSingleMode            = kingpin.Flag("single-mode", "run as single node").Default("0").Int()
	flagInit                  = kingpin.Flag("init", `init collector`).Bool()
	flagUpgrade               = kingpin.Flag("upgrade", ``).Bool()
	flagHost                  = kingpin.Flag("host", `eg. ip addr`).String()
	flagRemoteHost            = kingpin.Flag("remote-host", `data bridge addr`).Default("http://kodo.cloudcare.com").String()
	flagScrapeMetricInterval  = kingpin.Flag("scrape-metric-interval", "frequency to upload metric data").Default("60").Int()
	flagScrapeEnvInfoInterval = kingpin.Flag("scrape-env-info-interval", "frequency to upload env info data").Default("900").Int()
	flagTeamID                = kingpin.Flag("team-id", "User ID").String()
	flagCloudAssetID          = kingpin.Flag("cloud-asset-id", "cloud instance ID").String()
	flagAK                    = kingpin.Flag("ak", `Access Key`).String()
	flagSK                    = kingpin.Flag("sk", `Secret Key`).String()
	flagPort                  = kingpin.Flag("port", `web listen port`).Default("9100").Int()
	flagCfgFile               = kingpin.Flag("cfg", `configure file`).Default("cfg.yml").String()
	flagVersionInfo           = kingpin.Flag("version", "show version info").Bool()
	flagEnableAllCollectors   = kingpin.Flag("enable-all", "enable all collectors").Default("0").Int()
	flagInstallDir            = kingpin.Flag("install-dir", "install directory").Default("/usr/local/cloudcare").String()
	flagEnvCfg                = kingpin.Flag("env-cfg", "env-collector configure").Default("/usr/local/cloudcare/env.json").String()
)

func initCfg() error {
	cfg.Cfg.SingleMode = *flagSingleMode

	if *flagHost != "" {
		cfg.Cfg.Host = *flagHost
	}

	cfg.Cfg.RemoteHost = *flagRemoteHost
	cfg.Cfg.ScrapeMetricInterval = *flagScrapeMetricInterval
	cfg.Cfg.ScrapeEnvInfoInterval = *flagScrapeEnvInfoInterval
	cfg.Cfg.EnableAll = *flagEnableAllCollectors

	// unique-id 为必填参数
	if *flagTeamID == "" {
		log.Fatal("invalid team-id")
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
	cfg.Cfg.EnvCfgFile = *flagEnvCfg

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

	// init envinfo configure
	envinfo.OSQuerydPath = path.Join(*flagInstallDir, `osqueryd`)
	envinfo.Init(cfg.Cfg.EnvCfgFile)

	if cfg.Cfg.SingleMode == 1 {
		// metric 数据收集和上报
		getURLMetric := fmt.Sprintf("http://0.0.0.0:%d%s", cfg.Cfg.Port, *metricsPath)
		postURLMetric := fmt.Sprintf("%s%s", cfg.Cfg.RemoteHost, "/v1/write")
		if err := cloudcare.Start(postURLMetric, getURLMetric, cfg.Cfg.ScrapeMetricInterval); err != nil {
			panic(err)
		}

		// env info 收集器
		getURLEnv := fmt.Sprintf("http://0.0.0.0:%d%s", cfg.Cfg.Port, *envInfoPath)
		postURLEnv := fmt.Sprintf("%s%s", cfg.Cfg.RemoteHost, "/v1/write/env")
		if err := cloudcare.Start(postURLEnv, getURLEnv, cfg.Cfg.ScrapeEnvInfoInterval); err != nil {
			panic(err)
		}

		// TODO: 这些主动上报收集器, 并入集群模式时, 需要设计退出机制
	}

	http.Handle(*envInfoPath, handler.NewEnvInfoHandler())
	http.Handle(*metricsPath, handler.NewMetricHandler(!*disableExporterMetrics))
	http.HandleFunc(*metaPath, func(w http.ResponseWriter, r *http.Request) {
		j, err := json.Marshal(&cfg.Meta{
			CloudAssetID: cfg.Cfg.CloudAssetID,
			Host:         cfg.Cfg.Host,
		})
		if err != nil {
			log.Errorf("[error] %s, ignored", err.Error())
			fmt.Fprintf(w, err.Error())
		} else {
			fmt.Fprintf(w, string(j))
		}
	})

	listenAddress := fmt.Sprintf("0.0.0.0:%d", cfg.Cfg.Port)

	log.Infoln("Listening on", listenAddress)
	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		log.Fatal(err)
	}
}
