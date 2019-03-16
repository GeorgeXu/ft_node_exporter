package cloudcare

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/Go-zh/net/context/ctxhttp"
	"github.com/golang/protobuf/proto"
	"github.com/klauspost/compress/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/node_exporter/cfg"
	"github.com/prometheus/node_exporter/git"
	"github.com/prometheus/prometheus/prompb"

	config_util "github.com/prometheus/common/config"
)

const maxErrMsgLen = 256
const sleepMinute = 30

type Client struct {
	index   int // Used to differentiate clients in metrics.
	url     *config_util.URL
	client  *http.Client
	timeout time.Duration
}

// ClientConfig configures a Client.
type ClientConfig struct {
	URL              *config_util.URL
	Timeout          model.Duration
	HTTPClientConfig config_util.HTTPClientConfig
}

type KodoMsg struct {
	Code      int    `json:"code"`
	ErrorCode string `json:"errorCode"`
	Content   string `json:"content,omitempty"`
	Message   string `json:"message,omitempty"`
}

var ErrorCodeRejected = "carrier.kodo.rejected"

// NewClient creates a new Client.
func NewClient(index int, conf *ClientConfig) (*Client, error) {
	httpClient, err := config_util.NewClientFromConfig(conf.HTTPClientConfig, "remote_storage")
	if err != nil {
		return nil, err
	}

	return &Client{
		index:   index,
		url:     conf.URL,
		client:  httpClient,
		timeout: time.Duration(conf.Timeout),
	}, nil
}

type recoverableError struct {
	error
}

func calcSig(content []byte, contentType string,
	dateStr string, key string, method string, skVal string) string {
	h := md5.New()
	h.Write(content)

	cipherStr := h.Sum(nil)
	hex.EncodeToString(cipherStr)

	mac := hmac.New(sha1.New, []byte(skVal))
	mac.Write([]byte(method + "\n" +
		hex.EncodeToString(cipherStr) + "\n" +
		contentType + "\n" +
		dateStr + "\n" +
		key))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return sig
}

// Store sends a batch of samples to the HTTP endpoint.
var (
	storeTotal = 0
)

type issueResp struct {
	Code      int    `json:"code"`
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

func UploaderUidOK(uid string) bool {
	requrl := cfg.Cfg.RemoteHost + fmt.Sprintf("/v1/uploader-uid/check?uploader_uid=%s", cfg.Cfg.UploaderUID)
	httpReq, err := http.NewRequest(http.MethodGet, requrl, nil)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	date := time.Now().UTC().Format(http.TimeFormat)

	sig := calcSig(nil, "", date, cfg.Cfg.TeamID, http.MethodGet, cfg.DecodedSK)

	httpReq.Header.Set("X-Version", cfg.ProbeName+"/"+git.Version)
	httpReq.Header.Set("X-Team-Id", cfg.Cfg.TeamID)
	httpReq.Header.Set("X-Uploader-Uid", cfg.Cfg.UploaderUID)
	httpReq.Header.Set("X-Uploader-Ip", cfg.Cfg.Host)
	httpReq.Header.Set("X-Host-Name", HostName)
	httpReq.Header.Set("X-App-Name", cfg.ProbeName)
	httpReq.Header.Set("Date", date)
	httpReq.Header.Set("Authorization", "kodo "+cfg.Cfg.AK+":"+sig)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	if httpResp.StatusCode == 200 {
		return true
	}

	defer httpResp.Body.Close()

	resdata, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	var m issueResp

	err = json.Unmarshal(resdata, &m)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	log.Printf("[error] %s: %s", m.ErrorCode, m.Message)

	return false
}

func CreateIssueSourceOK() bool {

	requrl := cfg.Cfg.RemoteHost + "/v1/issue-source?is_ftagent=false"

	httpReq, err := http.NewRequest("POST", requrl, nil)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	date := time.Now().UTC().Format(http.TimeFormat)

	sig := calcSig(nil, "", date, cfg.Cfg.TeamID, http.MethodPost, cfg.DecodedSK)

	httpReq.Header.Set("X-Version", cfg.ProbeName+"/"+git.Version)
	httpReq.Header.Set("X-Team-Id", cfg.Cfg.TeamID)
	httpReq.Header.Set("X-Uploader-Uid", cfg.Cfg.UploaderUID)
	httpReq.Header.Set("X-Uploader-Ip", cfg.Cfg.Host)
	httpReq.Header.Set("X-Host-Name", HostName)
	httpReq.Header.Set("X-App-Name", cfg.ProbeName)
	httpReq.Header.Set("Date", date)
	httpReq.Header.Set("Authorization", "kodo "+cfg.Cfg.AK+":"+sig)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	if httpResp.StatusCode == 200 {
		return true
	}

	defer httpResp.Body.Close()

	resdata, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	var m issueResp

	err = json.Unmarshal(resdata, &m)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	log.Printf("[error] %+#v", m)

	return false
}

func (c *Client) Store(ctx context.Context, req *prompb.WriteRequest) error {

	data, err := proto.Marshal(req)
	if err != nil {
		//log.Printf("[error] %s", err.Error())
		return err
	}

	compressed := snappy.Encode(nil, data)
	httpReq, err := http.NewRequest("POST", c.url.String(), bytes.NewReader(compressed))
	if err != nil {
		// Errors from NewRequest are from unparseable URLs, so are not recoverable.
		//log.Printf("[error] %s", err.Error())
		return err
	}

	//log.Printf("[debug] snappy ratio: %d/%d=%f%%",len(compressed), len(data), (1.0-float64(len(compressed))/float64(len(data)))*100.0)

	contentType := "application/x-protobuf"
	contentEncode := "snappy"
	date := time.Now().UTC().Format(http.TimeFormat)

	sig := calcSig(compressed, contentType,
		date, cfg.Cfg.TeamID, http.MethodPost, cfg.DecodedSK)

	log.Println("HostName: ", HostName)

	httpReq.Header.Add("Content-Encoding", contentEncode)
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("X-Version", cfg.ProbeName+"/"+git.Version)
	httpReq.Header.Set("X-Team-Id", cfg.Cfg.TeamID)
	httpReq.Header.Set("X-Uploader-Uid", cfg.Cfg.UploaderUID)
	httpReq.Header.Set("X-Uploader-Ip", cfg.Cfg.Host)
	httpReq.Header.Set("X-Host-Name", HostName)
	httpReq.Header.Set("X-App-Name", cfg.ProbeName)
	httpReq.Header.Set("Date", date)
	httpReq.Header.Set("Authorization", "kodo "+cfg.Cfg.AK+":"+sig)
	httpReq = httpReq.WithContext(ctx)

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	httpResp, err := ctxhttp.Do(ctx, c.client, httpReq)
	if err != nil {
		// Errors from client.Do are from (for example) network errors, so are
		// recoverable.
		return recoverableError{err}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode/100 != 2 {

		scanner := bufio.NewScanner(io.LimitReader(httpResp.Body, maxErrMsgLen))
		line := []byte(`{"code": "", "errorCode": "", "message": ""}`)

		if scanner.Scan() {
			line = scanner.Bytes()
			var msg KodoMsg

			if err := json.Unmarshal(line, &msg); err != nil {
				// pass
			} else {
				if msg.ErrorCode == ErrorCodeRejected {
					log.Printf("[fatal] rejected by kodo: %s, process sleeping", msg.Message)
					sem.Close()
					time.Sleep(time.Duration(sleepMinute) * time.Minute)
					return nil
				}
			}
		}
		err = fmt.Errorf("server returned HTTP status %s: %s", httpResp.Status, string(line))
	}

	if httpResp.StatusCode/100 == 5 {
		return recoverableError{err}
	}
	return err
}

// Name identifies the client.
func (c Client) Name() string {
	return fmt.Sprintf("%d:%s", c.index, c.url)
}
