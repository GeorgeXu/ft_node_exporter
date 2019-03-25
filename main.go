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
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"path/filepath"

	"github.com/prometheus/common/version"
	"github.com/prometheus/node_exporter/fileinfo"
	"github.com/prometheus/node_exporter/git"
	"github.com/prometheus/node_exporter/handler"
	"github.com/prometheus/node_exporter/kv"
	"github.com/prometheus/node_exporter/utils"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	kvJsonUrlPath = kingpin.Flag("web.telemetry-env-info-path", "Path under which to expose env info.").Default("/kvs/json").String()
	//kvUrlPath       = kingpin.Flag("web.telemetry-env-info-path", "Path under which to expose env info.").Default("/kvs").String()
	fileinfoUrlPath = kingpin.Flag("web.telemetry-file-info-path", "Path under which to expose file info.").Default("/fileinfos").String()

	disableExporterMetrics = kingpin.Flag("web.disable-exporter-metrics", "Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).").Bool()

	flagBindAddr = kingpin.Flag("bind-addr", `http server bind addr`).Default(`localhost:9100`).String()

	flagKvCfg       = kingpin.Flag("env-cfg", "env-collector configure").Default(`/usr/local/cloudcare/ft_node_exporter/kv.json`).String()
	flagFileinfoCfg = kingpin.Flag("fileinfo-cfg", "cfg-collector configure").Default(`/usr/local/cloudcare/ft_node_exporter/fileinfo.json`).String()

	flagEnableAllCollectors = kingpin.Flag("enable-all", "enable all collectors").Default(`1`).Int()

	flagVersionInfo = kingpin.Flag("version", "show version info").Bool()
	flagInstallDir  = kingpin.Flag("install-dir", "install directory").Default(`/usr/local/cloudcare/ft_node_exporter/`).String()
	AppName         = "ft_node_exporter"
)

func main() {

	//log.SetFlags(log.Llongfile | log.LstdFlags)
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	version.Version = git.Version
	version.BuildDate = git.BuildAt

	logfilepath := fmt.Sprintf("%s%s.log", *flagInstallDir, AppName)
	rw, err := utils.SetLog(logfilepath)
	if err != nil {
		log.Fatal(err)
	}
	defer rw.Close()

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

	// init kv configure
	kv.OSQuerydPath = filepath.Join(*flagInstallDir, `osqueryd`)
	kv.Init(*flagKvCfg)
	fileinfo.Init(*flagFileinfoCfg)

	http.Handle(*kvJsonUrlPath, handler.NewKvHandler())
	http.Handle("/kvs", handler.NewKvHandler())
	http.Handle(*fileinfoUrlPath, handler.NewFileInfoHandler())
	http.Handle(*metricsPath, handler.NewMetricHandler(!*disableExporterMetrics))

	l, err := net.Listen(`tcp`, *flagBindAddr)
	if err != nil {
		log.Fatalf("[fatal] %s", err.Error())
	}

	if err := utils.DumpPID(*flagInstallDir, AppName); err != nil {
		log.Fatalf("dump pid failed: %s", err)
	}

	defer l.Close()
	if err := http.Serve(l, nil); err != nil {
		log.Println("[fatal]", err)
	}
}
