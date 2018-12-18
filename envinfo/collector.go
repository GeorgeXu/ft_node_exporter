package envinfo

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// 通过 osquery 收集各种主机数据

// osquery 各种表以及系统数据映射
/*

	用户列表					                   : users
	用户分组列表			                   : groups
	主机名和IP配置文件                   : etc_hosts
	DNS解析配置文件		                   : dns_resolvers
	可执行sudo的用户列表                 : sudoers
	内核参数配置                         : system_controls
	Linux PAM认证下可打开的文件句柄数限制: ulimit_info
	定时任务                             : crontab
	远程访问白名单配置                   : 不支持
	远程访问黑名单配置                   : 不支持
	有效登陆shell的列表                  : 不支持
	用户登录后终端显示消息配置           : 不支持
	用户本地登录前终端显示消息置         : 不支持
	用户远程登录前终端显示消息配置       : 不支持
	自启动脚本                           : 不支持
	不同运行级别自启动脚本               : 不支持
	当前主机文件系统的相关信息           : 不支持
	用户环境配置信息                     : 不支持
	用户bash shell配置信息               : 不支持
	SELinux(安全增加)配置文件            : 未知
	系统开启启动配置                     : 不支持
	主机用户列表(含加密密码)             : 不支持
	sshd 配置					                   : 不支持
*/

const namespace = "envinfo"

var (
	OSQuerydPath = ""
	// run osquery:  ./osqueryd -S --json 'select * from users'
	//   -S: run as shell mode
	//   --json: output result in json format

	factories      = make(map[string]func() (Collector, error))
	collectorState = make(map[string]bool)

	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"envinfo: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"envinfo: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

type Collector interface {
	Update(ch chan<- prometheus.Metric) error
}

func registerCollector(collector string, isDefaultEnabled bool, factory func() (Collector, error)) {
	collectorState[collector] = isDefaultEnabled
	factories[collector] = factory
}

type EnvInfoCollector struct {
	Collectors map[string]Collector
}

func NewEnvInfoCollector(filters ...string) (*EnvInfoCollector, error) {
	f := make(map[string]bool)
	for _, filter := range filters {
		enabled, exist := collectorState[filter]
		if !exist {
			return nil, fmt.Errorf("missing collector: %s", filter)
		}
		if !enabled {
			return nil, fmt.Errorf("disabled collector: %s", filter)
		}
		f[filter] = true
	}

	collectors := make(map[string]Collector)
	for key, enabled := range collectorState {
		if enabled {
			collector, err := factories[key]() // call NewxxxCollector()
			if err != nil {
				continue
			}
			if len(f) == 0 || f[key] {
				collectors[key] = collector
			}
		}
	}
	return &EnvInfoCollector{Collectors: collectors}, nil
}

func (c EnvInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

func (c EnvInfoCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Collectors))

	log.Debugf("envinfo try collect...")

	for name, _c := range c.Collectors {
		go func(name string, ec Collector) {
			execute(name, ec, ch)
			wg.Done()
		}(name, _c)
	}
	wg.Wait()
}

func execute(name string, c Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		log.Errorf("ERROR: %s collector failed after %fs: %s", name, duration.Seconds(), err)
		success = 0
	} else {
		log.Debugf("OK: %s collector succeeded after %fs.", name, duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

func doCat(path string) (string, error) {

	cmd := exec.Command(`cat`, []string{path}...)

	log.Debugf("cat: %s", path)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(out), nil
}

func doQuery(sql string) (string, error) {
	cmd := exec.Command(OSQuerydPath, []string{`-S`, `--json`, sql}...)

	log.Debugf("osquery: %s", sql)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(out), nil
}
