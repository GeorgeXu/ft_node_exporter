package cfg

import (
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"

	"github.com/prometheus/node_exporter/collector"
	"github.com/prometheus/node_exporter/utils"
)

type Config struct {
	TeamID          string `yaml:"team_id"`
	UploaderUID     string `yaml:"uploader_uid"`
	AK              string `yaml:"ak"`
	SK              string `yaml:"sk"`
	BindAddr        string `yaml:"bind_addr"`
	SingleMode      int    `yaml:"single_mode"`
	Host            string `yaml:"host"`
	GroupName       string `yaml:"group_name"`
	RemoteHost      string `yaml:"remote_host"`
	EnableAll       int    `yaml:"enable_all"`
	EnvCfgFile      string `yaml:"env_cfg_file"`
	FileInfoCfgFile string `yaml:"fileinfo_cfg_file"`
	Provider        string `yaml:"provider"`

	ScrapeMetricInterval   int `yaml:"scrap_metric_interval"`
	ScrapeEnvInfoInterval  int `yaml:"scrap_env_info_interval"`
	ScrapeFileInfoInterval int `yaml:"scrap_file_info_interval"`

	Collectors map[string]bool `yaml:"collectors"`
	QueueCfg   map[string]int  `yaml:"queue_cfg"`
}

type Meta struct {
	UploaderUID string `json:"uploader_uid"`
	GroupName   string `json:"group_name"`
}

const (
	InstallDir     = `/usr/local/cloudcare/profwang_probe/`
	ProbeName      = `profwang_probe`
	DefaultCfgPath = InstallDir + ProbeName + ".yml"
)

var (
	Cfg = Config{
		QueueCfg: map[string]int{
			`batch_send_deadline`:  5,
			`capacity`:             10000,
			`max_retries`:          3,
			`max_samples_per_send`: 100,
		},
		Host:                   `default`,
		RemoteHost:             `http://kodo.cloudcare.com`,
		SingleMode:             1,
		EnableAll:              1,
		BindAddr:               `localhost:9100`,
		EnvCfgFile:             InstallDir + "env.json",
		FileInfoCfgFile:        InstallDir + "fileinfo.json",
		ScrapeMetricInterval:   60000,
		ScrapeEnvInfoInterval:  900000,
		ScrapeFileInfoInterval: 86400000,
	}

	DecodedSK = ""
)

// 导入 @f 中的配置
func LoadConfig(f string) error {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		log.Printf("[error] open %s failed: %s", f, err.Error())
		return err
	}

	if err := yaml.Unmarshal(data, &Cfg); err != nil {
		log.Printf("[error] yaml load %s failed: %s", f, err.Error())
		return err
	}

	if Cfg.Host == "" {
		Cfg.Host = "default"
	}

	if Cfg.SK != "" {
		DecodedSK = string(utils.XorDecode(Cfg.SK))
	}

	if Cfg.EnableAll == 1 {
		// 开启所有收集器
		for k, _ := range Cfg.Collectors {
			collector.SetCollector(k, true)
		}
	} else {
		// 将配置中的开关设置到 collector 模块中
		for k, v := range Cfg.Collectors {
			collector.SetCollector(k, v)
		}
	}

	return nil
}

// 当前配置写入配置文件
func DumpConfig(f string) error {
	c, err := yaml.Marshal(&Cfg)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(f, c, 0644)
}
