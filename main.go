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
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	"github.com/prometheus/node_exporter/cfg"
	"github.com/prometheus/node_exporter/cloudcare"
	"github.com/prometheus/node_exporter/collector"
	"github.com/prometheus/node_exporter/envinfo"
	"github.com/prometheus/node_exporter/fileinfo"
	"github.com/prometheus/node_exporter/git"
	"github.com/prometheus/node_exporter/handler"
	"github.com/prometheus/node_exporter/utils"
	uuid "github.com/satori/go.uuid"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	metricsPath  = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	envInfoPath  = kingpin.Flag("web.telemetry-env-info-path", "Path under which to expose env info.").Default("/env_infos").String()
	fileInfoPath = kingpin.Flag("web.telemetry-file-info-path", "Path under which to expose file info.").Default("/file_infos").String()

	cfgAPI = kingpin.Flag("web.meta-path", "Path under which to expose meta info.").Default("/config").String()

	disableExporterMetrics = kingpin.Flag("web.disable-exporter-metrics", "Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).").Bool()

	flagSingleMode  = kingpin.Flag("single-mode", "run as single node").Default(fmt.Sprintf("%d", cfg.Cfg.SingleMode)).Int()
	flagInit        = kingpin.Flag("init", `init collector`).Bool()
	flagUpgrade     = kingpin.Flag("upgrade", ``).Bool()
	flagHost        = kingpin.Flag("host", `eg. ip addr`).Default().String()
	flagUploaderUID = kingpin.Flag("uploader-uid", `uuid`).Default().String()

	flagGroupName = kingpin.Flag(`group-name`, `group name`).Default(cfg.Cfg.GroupName).String()

	flagRemoteHost = kingpin.Flag("remote-host", `data bridge addr`).Default(cfg.Cfg.RemoteHost).String()

	flagScrapeMetricInterval = kingpin.Flag("scrape-metric-interval",
		"frequency to upload metric data(ms)").Default(fmt.Sprintf("%d", cfg.Cfg.ScrapeMetricInterval)).Int()
	flagScrapeEnvInfoInterval = kingpin.Flag("scrape-env-info-interval",
		"frequency to upload env info data(ms)").Default(fmt.Sprintf("%d", cfg.Cfg.ScrapeEnvInfoInterval)).Int()
	flagScrapeFileInfoInterval = kingpin.Flag("scrape-file-info-interval",
		"frequency to upload file info data(ms)").Default(fmt.Sprintf("%d", cfg.Cfg.ScrapeFileInfoInterval)).Int()

	flagPort = kingpin.Flag("port", `web listen port`).Int()

	flagEnvCfg      = kingpin.Flag("env-cfg", "env-collector configure").Default(cfg.Cfg.EnvCfgFile).String()
	flagFileInfoCfg = kingpin.Flag("fileinfo-cfg", "fileinfo-collector configure").Default(cfg.Cfg.FileInfoCfgFile).String()

	flagEnableAllCollectors = kingpin.Flag("enable-all", "enable all collectors").Default(fmt.Sprintf("%d", cfg.Cfg.EnableAll)).Int()

	flagTeamID      = kingpin.Flag("team-id", "User ID").String()
	flagAK          = kingpin.Flag("ak", `Access Key`).String()
	flagSK          = kingpin.Flag("sk", `Secret Key`).String()
	flagCfgFile     = kingpin.Flag("cfg", `configure file`).Default(cfg.DefaultCfgPath).String()
	flagVersionInfo = kingpin.Flag("version", "show version info").Bool()
	flagInstallDir  = kingpin.Flag("install-dir", "install directory").Default(cfg.InstallDir).String()

	flagProvider = kingpin.Flag("provider", "cloud service provider").Default("aliyun").String()

	flagEnSK = kingpin.Flag("en-sk", ``).String()
	flagDeSK = kingpin.Flag("de-sk", ``).String()
)

func initCfg() error {

	cfg.Cfg.SingleMode = *flagSingleMode

	if *flagHost != "" {
		cfg.Cfg.Host = *flagHost
	}

	cfg.Cfg.RemoteHost = *flagRemoteHost
	cfg.Cfg.ScrapeMetricInterval = *flagScrapeMetricInterval
	cfg.Cfg.ScrapeEnvInfoInterval = *flagScrapeEnvInfoInterval
	cfg.Cfg.ScrapeFileInfoInterval = *flagScrapeFileInfoInterval
	cfg.Cfg.EnableAll = *flagEnableAllCollectors

	// 单机模式下，team-id 为必填参数
	if *flagTeamID != "" {
		cfg.Cfg.TeamID = *flagTeamID
	} else {
		if *flagSingleMode == 1 {
			log.Fatal("--team-id required")
		}
	}

	if *flagAK == "" {
		log.Fatalln("[fatal] invalid ak")
	} else {
		cfg.Cfg.AK = *flagAK
	}

	if *flagSK == "" {
		log.Fatalln("[fatal] invalid sk")
	} else {
		cfg.Cfg.SK = utils.XorEncode(*flagSK)
		cfg.DecodedSK = *flagSK
	}

	cfg.Cfg.Port = *flagPort
	if cfg.Cfg.Port == 0 {
		cfg.Cfg.Port = 9100
	}
	cfg.Cfg.EnvCfgFile = *flagEnvCfg
	cfg.Cfg.FileInfoCfgFile = *flagFileInfoCfg
	cfg.Cfg.Provider = *flagProvider

	cfg.Cfg.Collectors = collector.ListAllCollectors()

	if cfg.Cfg.EnableAll == 1 {
		for k := range cfg.Cfg.Collectors {
			cfg.Cfg.Collectors[k] = true
		}
	}

	if *flagUploaderUID != "" {
		cfg.Cfg.UploaderUID = *flagUploaderUID
		if !cloudcare.UploaderUidOK(*flagUploaderUID) {
			os.Exit(-1)
		}
	} else {
		// 客户端自行生成 ID, 而不是 kodo 下发
		uid, err := uuid.NewV4()
		if err != nil {
			log.Fatal(err)
		}

		cfg.Cfg.UploaderUID = fmt.Sprintf("uid-%s", uid.String())
	}

	return cfg.DumpConfig(*flagCfgFile)
}

func probeCheck() error {
	url := fmt.Sprintf("%s/v1/probe/check?team_id=%s&probe=%s&uploader_uid=%s",
		cfg.Cfg.RemoteHost, cfg.Cfg.TeamID, cfg.ProbeName, cfg.Cfg.UploaderUID)

	resp, err := http.Get(url)
	if err != nil { // 可能网络不通
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("[fatal] check failed: %s", string(body))
	}

	msg := map[string]string{}
	if err := json.Unmarshal(body, &msg); err != nil {
		log.Fatal(err)
	}

	if msg[`error`] != "" {
		return fmt.Errorf(msg[`error`])
	}

	return nil
}

func main() {

	//log.SetFlags(log.Llongfile | log.LstdFlags)
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	logfilepath := fmt.Sprintf("%s%s.log", cfg.InstallDir, cfg.ProbeName)
	rw, err := cloudcare.SetLog(logfilepath)
	if err != nil {
		log.Fatal(err)
	}
	defer rw.Close()

	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if len(*flagEnSK) > 0 {
		enSk := utils.XorEncode(*flagEnSK)
		fmt.Println(enSk)
		return
	}

	if len(*flagDeSK) > 0 {
		deSk := utils.XorDecode(*flagDeSK)
		fmt.Println(string(deSk))
		return
	}

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
		// 只是更新二进制, 不改变之前的配置
		return
	}

	if err := cfg.LoadConfig(*flagCfgFile); err != nil {
		log.Fatalf("[fatal] load config failed: %s", err)
	}

	cfg.DumpConfig(*flagCfgFile) // load 过程中可能会修改 cfg.Cfg, 需重新写入

	if cfg.Cfg.SingleMode > 0 {
		// 单机模式下，无脑调用 create-issue-source 接口
		if !cloudcare.CreateIssueSourceOK() {
			os.Exit(-1)
		}
	}

	// init envinfo configure
	envinfo.OSQuerydPath = filepath.Join(*flagInstallDir, `osqueryd`)
	envinfo.Init(cfg.Cfg.EnvCfgFile)
	fileinfo.Init(cfg.Cfg.FileInfoCfgFile)

	log.Println(fmt.Sprintf("[info] start on %d ...", cfg.Cfg.Port))

	if cfg.Cfg.SingleMode == 1 {
		// metric 数据收集和上报
		getURLMetric := fmt.Sprintf("http://localhost:%d%s", cfg.Cfg.Port, *metricsPath)

		log.Printf("[debug] metric url: %s", getURLMetric)

		postURLMetric := fmt.Sprintf("%s%s", cfg.Cfg.RemoteHost, "/v1/write")

		if err := cloudcare.Start(postURLMetric, getURLMetric, int64(cfg.Cfg.ScrapeMetricInterval)); err != nil {
			log.Fatalf("[fatal] %s", err)
		}

		// env info 收集器
		getURLEnv := fmt.Sprintf("http://localhost:%d%s?format=json", cfg.Cfg.Port, *envInfoPath)

		log.Printf("[debug] env-info url: %s", getURLEnv)

		postURLEnv := fmt.Sprintf("%s%s", cfg.Cfg.RemoteHost, "/v1/write/env")
		if err := cloudcare.Start(postURLEnv, getURLEnv, int64(cfg.Cfg.ScrapeEnvInfoInterval)); err != nil {
			log.Fatalf("[fatal] %s", err)
		}

		// file info 收集器
		getURLFile := fmt.Sprintf("http://localhost:%d%s", cfg.Cfg.Port, *fileInfoPath)

		log.Printf("[debug] env-info url: %s", getURLFile)

		postURLFile := fmt.Sprintf("%s%s", cfg.Cfg.RemoteHost, "/v1/write/env")
		if err := cloudcare.Start(postURLFile, getURLFile, int64(cfg.Cfg.ScrapeFileInfoInterval)); err != nil {
			log.Fatalf("[fatal] %s", err)
		}

		// TODO: 这些主动上报收集器, 并入集群模式时, 需要设计退出机制
	}

	http.Handle(*envInfoPath, handler.NewEnvInfoHandler())
	http.Handle(*fileInfoPath, handler.NewFileInfoHandler())
	http.Handle(*metricsPath, handler.NewMetricHandler(!*disableExporterMetrics))

	http.HandleFunc(*cfgAPI, func(w http.ResponseWriter, r *http.Request) {

		if cfg.Cfg.GroupName == "" { // GroupName 默认为探针运行所在机器的 hostname

			hostName, err := os.Hostname()
			if err != nil {
				log.Printf("[error] %s, ignored", err.Error())
			} else {
				cloudcare.HostName = hostName
			}

			cfg.Cfg.GroupName = hostName
		}

		j, err := json.Marshal(&cfg.Meta{
			UploaderUID: cfg.Cfg.UploaderUID,
			GroupName:   cfg.Cfg.GroupName,
		})

		if err != nil {
			log.Printf("[error] %s, ignored", err.Error())
			fmt.Fprintf(w, err.Error())
		} else {

			w.Header().Set(`Content-Type`, `application/json`)
			w.Header().Set(`Content-Length`, fmt.Sprintf("%d", len(j)))
			fmt.Fprintf(w, string(j))
		}
	})

	listenAddress := fmt.Sprintf("localhost:%d", cfg.Cfg.Port)
	l, err := net.Listen(`tcp`, listenAddress)
	if err != nil {
		log.Fatalf("[fatal] %s", err.Error())
	}

	if err := cloudcare.DumpPID(); err != nil {
		log.Fatalf("dump pid faile: %s", err)
	}

	defer l.Close()
	if err := http.Serve(l, nil); err != nil {
		log.Println("[fatal]", err)
	}
}
