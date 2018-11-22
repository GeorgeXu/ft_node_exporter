package cloudcare

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Go-zh/net/context/ctxhttp"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/golang/protobuf/proto"
	"github.com/klauspost/compress/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"

	config_util "github.com/prometheus/common/config"
)

const maxErrMsgLen = 256

type Client struct {
	index   int // Used to differentiate clients in metrics.
	url     *config_util.URL
	client  *http.Client
	timeout time.Duration
	logger  log.Logger
}

// ClientConfig configures a Client.
type ClientConfig struct {
	URL              *config_util.URL
	Timeout          model.Duration
	HTTPClientConfig config_util.HTTPClientConfig
}

// NewClient creates a new Client.
func NewClient(index int, l log.Logger, conf *ClientConfig) (*Client, error) {
	httpClient, err := config_util.NewClientFromConfig(conf.HTTPClientConfig, "remote_storage")
	if err != nil {
		return nil, err
	}

	return &Client{
		index:   index,
		url:     conf.URL,
		client:  httpClient,
		timeout: time.Duration(conf.Timeout),
		logger:  l,
	}, nil
}

type recoverableError struct {
	error
}

// Store sends a batch of samples to the HTTP endpoint.
func (c *Client) Store(ctx context.Context, req *prompb.WriteRequest) error {
	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	level.Info(c.logger).Log("msg", "----send", len(data), len(data))

	compressed := snappy.Encode(nil, data)
	httpReq, err := http.NewRequest("POST", c.url.String(), bytes.NewReader(compressed))
	if err != nil {
		// Errors from NewRequest are from unparseable URLs, so are not
		// recoverable.
		return err
	}

	// level.Debug(c.logger).Log("msg", "snappy ratio", 1.0-float64(len(compressed))/float64(len(data)))
	level.Debug(c.logger).Log("msg", "snappy ratio", len(compressed), len(data))

	httpReq.Header.Add("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	httpReq.Header.Set("X-Prometheus-Key", thecfg.GlobalConfig.Key)
	httpReq = httpReq.WithContext(ctx)

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	fmt.Println(c.url.String())

	httpResp, err := ctxhttp.Do(ctx, c.client, httpReq)
	if err != nil {
		// Errors from client.Do are from (for example) network errors, so are
		// recoverable.
		return recoverableError{err}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(httpResp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s: %s", httpResp.Status, line)
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

// func (c *client) Store(ctx context.Context, req *prompb.WriteRequest) error {
// 	data, err := proto.Marshal(req)
// 	if err != nil {
// 		return err
// 	}

// 	compressed := snappy.Encode(nil, data)
// 	httpReq, err := http.NewRequest("POST", c.url.String(), bytes.NewReader(compressed))
// 	if err != nil {
// 		// Errors from NewRequest are from unparseable URLs, so are not
// 		// recoverable.
// 		return err
// 	}

// 	// level.Debug(c.logger).Log("msg", "snappy ratio", 1.0-float64(len(compressed))/float64(len(data)))
// 	//level.Debug(c.logger).Log("msg", "snappy ratio", len(compressed), len(data))

// 	httpReq.Header.Add("Content-Encoding", "snappy")
// 	httpReq.Header.Set("Content-Type", "application/x-protobuf")
// 	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
// 	httpReq.Header.Set("X-Prometheus-Key", cfg.key)
// 	httpReq = httpReq.WithContext(ctx)

// 	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
// 	defer cancel()

// 	httpResp, err := ctxhttp.Do(ctx, c.client, httpReq)

// 	if err != nil {
// 		// Errors from client.Do are from (for example) network errors, so are
// 		// recoverable.
// 		return recoverableError{err}
// 	}
// 	defer httpResp.Body.Close()

// 	if httpResp.StatusCode/100 != 2 {
// 		scanner := bufio.NewScanner(io.LimitReader(httpResp.Body, maxErrMsgLen))
// 		line := ""
// 		if scanner.Scan() {
// 			line = scanner.Text()
// 		}
// 		err = fmt.Errorf("server returned HTTP status %s: %s", httpResp.Status, line)
// 	}
// 	if httpResp.StatusCode/100 == 5 {
// 		return recoverableError{err}
// 	}
// 	return err
// }
