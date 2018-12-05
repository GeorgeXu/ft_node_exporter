package cloudcare

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/prometheus/config"
)

type CCCmdArgs struct {
	Host       string
	UniqueID   string
	InstanceID string
	AK         string
	SK         string
	Port       string
}

var remoteWriteUrl *url.URL

func InitHandler(c *CCCmdArgs, singleton bool) error {

	if singleton {
		if c.UniqueID == "" || c.AK == "" || c.SK == "" || c.InstanceID == "" {
			return fmt.Errorf("invalid arguments")
		}
	} else {
		if c.InstanceID == "" {
			return fmt.Errorf("instance_id required")
		}
	}

	var cfg config.Config

	if c.Host != "" {
		cfg.GlobalConfig.Host = c.Host
	}
	if c.UniqueID != "" {
		cfg.GlobalConfig.UniqueID = c.UniqueID
	}
	if c.AK != "" {
		cfg.GlobalConfig.AK = c.AK
	}
	if c.SK != "" {
		cfg.GlobalConfig.SK = xorEncode(c.SK)
	}
	if c.Port != "" {
		cfg.GlobalConfig.Port = c.Port
	}
	cfg.GlobalConfig.InstanceID = c.InstanceID

	// if singleton {
	// 	rwCfg := &config.RemoteWriteConfig{}
	// 	u, err := url.Parse(cfg.GlobalConfig.Host)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	rwCfg.URL = &config_util.URL{u}

	// 	cfg.RemoteWriteConfigs = []*config.RemoteWriteConfig{rwCfg}
	// }

	return cfg.StoreToFile("cfg.yml")
}

func loop(s *Storage, scrapeurl string) {

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

func reloadConfig(filename string) (err error) {
	conf, err := config.LoadFile(filename)
	if err != nil {
		//fmt.Printf("load err: %s", err)
		return fmt.Errorf("couldn't load configuration (--config.file=%q): %v", filename, err)
	}

	thecfg = conf

	thecfg.GlobalConfig.SK = string(xorDecode(thecfg.GlobalConfig.SK))
	fmt.Printf("sk: %s", thecfg.GlobalConfig.SK)

	return nil
}

var remoteStorage *Storage
var thecfg *config.Config
var chStop chan struct{}

func ReloadConfig(f string) error {
	return reloadConfig(f)
}

func Start(remotehost string, scrapehost string) error {

	u, err := url.Parse(remotehost)
	if err != nil {
		return err
	}

	remoteWriteUrl = u

	var l promlog.AllowedLevel
	l.Set("info")
	logger := promlog.New(l)

	chStop = make(chan struct{})

	RemoteFlushDeadline := time.Duration(60 * time.Second)

	remoteStorage = NewStorage(log.With(logger, "component", "remote"), nil, time.Duration(RemoteFlushDeadline))

	if err := remoteStorage.ApplyConfig(thecfg); err != nil {
		return err
	}

	go func() {
		time.Sleep(1 * time.Second)
		loop(remoteStorage, scrapehost)
	}()

	return nil
}

func GetListenPort() string {
	p := thecfg.GlobalConfig.Port
	if p == "" {
		p = "9100"
	}
	return p
}

func GetDataBridgeUrl() *url.URL {
	return remoteWriteUrl
}

var xorkeys = []byte{0xbb, 0x74, 0x24, 0xa5, 0xba, 0x5a, 0xa, 0x8c, 0x65, 0x61, 0xdf, 0x57, 0xa1, 0x3c, 0xfb, 0xe9, 0x89, 0x12, 0xcb, 0x5a, 0xd2, 0x70, 0xf3, 0x82, 0x67, 0xdd, 0x5c, 0x8a, 0xec, 0x77, 0xcf, 0x48, 0x39, 0x1c, 0xe, 0xab, 0xee, 0xe, 0x16, 0xe8, 0x2c, 0xab, 0xf2, 0x61, 0xfc, 0xc7, 0xfd, 0x1c, 0x58, 0xfc, 0xe7, 0x4f, 0x70, 0xed, 0xc8, 0xf1, 0x5f, 0x36, 0x18, 0x3c, 0x29, 0x38, 0x27, 0xc1, 0xbc, 0x29, 0x3, 0x89, 0xcb, 0xbe, 0xc7, 0xc8, 0xce, 0xb3, 0x7d, 0x7d, 0xe1, 0x84, 0x74, 0xd, 0x1c, 0x66, 0xb6, 0x86, 0xbc, 0xb, 0x33, 0x1, 0x17, 0x93, 0xd3, 0x82, 0xb7, 0xb0, 0x96, 0xe3, 0xd6, 0xef, 0xc4, 0xa1, 0xf7, 0xb0, 0x6e, 0xd, 0x55, 0x2e, 0x3e, 0x25, 0x4c, 0xf7, 0xc6, 0xeb, 0x63, 0x8c, 0x88, 0x69, 0xf5, 0x86, 0x6a, 0x56, 0xc1, 0xaf, 0x46, 0xbf, 0x6f, 0x35, 0xfc, 0x90}

func xorEncode(sk string) string {

	r := rand.New(rand.NewSource(time.Now().Unix()))

	var msg bytes.Buffer

	msg.WriteByte(byte(len(sk)))
	msg.Write([]byte(sk))
	for {
		if msg.Len() >= 128 {
			break
		}
		msg.WriteByte(byte(r.Intn(255)))
	}

	var en bytes.Buffer
	for index := 0; index < msg.Len(); index++ {
		en.WriteByte(msg.Bytes()[index] ^ xorkeys[index])
	}

	return base64.StdEncoding.EncodeToString(en.Bytes())
}

func xorDecode(endata string) []byte {
	data, err := base64.StdEncoding.DecodeString(endata)
	if err != nil {
		return nil
	}
	length := data[0] ^ xorkeys[0]

	var dedata bytes.Buffer
	for index := 0; index < 128; index++ {
		dedata.WriteByte(data[index] ^ xorkeys[index])
	}
	return dedata.Bytes()[1 : 1+length]
}
