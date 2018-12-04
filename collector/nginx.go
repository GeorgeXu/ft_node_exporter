package collector

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	ngxSubSystem = "nginx"
)

// nginxClient allows you to fetch NGINX metrics from the stub_status page.
type nginxClient struct {
	apiEndpoint string
	httpClient  *http.Client
}

// nginxCollector collects NGINX metrics. It implements prometheus.Collector interface.
type nginxCollector struct {
	client *nginxClient
	mutex  sync.Mutex

	connectionsActive   *prometheus.Desc
	connectionsAccepted *prometheus.Desc
	connectionsHandled  *prometheus.Desc
	connectionsReading  *prometheus.Desc
	connectionsWriting  *prometheus.Desc
	connectionsWaiting  *prometheus.Desc
	httpRequestsTotal   *prometheus.Desc
}

// StubStats represents NGINX stub_status metrics.
type StubStats struct {
	Connections StubConnections
	Requests    int64
}

// StubConnections represents connections related metrics.
type StubConnections struct {
	Active   int64
	Accepted int64
	Handled  int64
	Reading  int64
	Writing  int64
	Waiting  int64
}

var (
	nginxScrapURI = kingpin.Flag(
		"collector.nginx.scrap-url",
		"Address on where to scrap Nginx stub info.").Default("http://127.0.0.1:8080/stub_status").String()

	nginxSSLVerify = kingpin.Flag(
		"collector.nginx.ssl-verify",
		"Perform SSL certificate verification.").Default("true").Bool()
)

func init() {
	registerCollector(ngxSubSystem, defaultDisabled, NewNginxCollector)
}

func NewNginxCollector() (Collector, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !*nginxSSLVerify},
	}

	client, err := NewNginxClient(&http.Client{Transport: tr}, *nginxScrapURI)
	if err != nil {
		return nil, err
	}

	return &nginxCollector{
		client: client,

		connectionsActive: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, ngxSubSystem, "connections_active"),
			"Active client connections", nil, nil),

		connectionsAccepted: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, ngxSubSystem, "connections_accepted"),
			"Accepted client connections", nil, nil),

		connectionsHandled: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, ngxSubSystem, "connections_handled"),
			"Handled client connections", nil, nil),

		connectionsReading: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, ngxSubSystem, "connections_reading"),
			"Connections where NGINX is reading the request header", nil, nil),

		connectionsWriting: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, ngxSubSystem, "connections_writing"),
			"Connections where NGINX is writing the response back to the client", nil, nil),

		connectionsWaiting: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, ngxSubSystem, "connections_waiting"),
			"Idle client connections", nil, nil),

		httpRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, ngxSubSystem, "http_requests_total"),
			"Total http requests", nil, nil),
	}, nil
}

// Collect fetches metrics from NGINX and sends them to the provided channel.
func (c *nginxCollector) Update(ch chan<- prometheus.Metric) error {
	c.mutex.Lock() // To protect metrics from concurrent collects
	defer c.mutex.Unlock()

	stats, err := c.client.GetStubStats()
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		return err
	}

	ch <- prometheus.MustNewConstMetric(c.connectionsActive, prometheus.GaugeValue, float64(stats.Connections.Active))
	ch <- prometheus.MustNewConstMetric(c.connectionsAccepted, prometheus.CounterValue, float64(stats.Connections.Accepted))
	ch <- prometheus.MustNewConstMetric(c.connectionsHandled, prometheus.CounterValue, float64(stats.Connections.Handled))
	ch <- prometheus.MustNewConstMetric(c.connectionsReading, prometheus.GaugeValue, float64(stats.Connections.Reading))
	ch <- prometheus.MustNewConstMetric(c.connectionsWriting, prometheus.GaugeValue, float64(stats.Connections.Writing))
	ch <- prometheus.MustNewConstMetric(c.connectionsWaiting, prometheus.GaugeValue, float64(stats.Connections.Waiting))
	ch <- prometheus.MustNewConstMetric(c.httpRequestsTotal, prometheus.CounterValue, float64(stats.Requests))
	return nil
}

func newGlobalMetric(namespace string, metricName string, docString string) *prometheus.Desc {
	return prometheus.NewDesc(namespace+"_"+metricName, docString, nil, nil)
}

// NewNginxClient creates an nginxClient.
func NewNginxClient(httpClient *http.Client, apiEndpoint string) (*nginxClient, error) {
	client := &nginxClient{
		apiEndpoint: apiEndpoint,
		httpClient:  httpClient,
	}

	if _, err := client.GetStubStats(); err != nil {
		return nil, fmt.Errorf("Failed to create nginxClient: %v", err)
	}

	return client, nil
}

// fetches the stub_status metrics.
func (client *nginxClient) GetStubStats() (*StubStats, error) {
	resp, err := client.httpClient.Get(client.apiEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get %v: %v", client.apiEndpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected %v response, got %v", http.StatusOK, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read the response body: %v", err)
	}

	var stats StubStats
	err = parseStubStats(body, &stats)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body %q: %v", string(body), err)
	}

	return &stats, nil
}

func parseStubStats(data []byte, stats *StubStats) error {
	dataStr := string(data)

	parts := strings.Split(dataStr, "\n")
	if len(parts) != 5 {
		return fmt.Errorf("invalid input %q", dataStr)
	}

	activeConsParts := strings.Split(strings.TrimSpace(parts[0]), " ")
	if len(activeConsParts) != 3 {
		return fmt.Errorf("invalid input for active connections %q", parts[0])
	}

	actCons, err := strconv.ParseInt(activeConsParts[2], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid input for active connections %q: %v", activeConsParts[2], err)
	}
	stats.Connections.Active = actCons

	miscParts := strings.Split(strings.TrimSpace(parts[2]), " ")
	if len(miscParts) != 3 {
		return fmt.Errorf("invalid input for connections and requests %q", parts[2])
	}

	acceptedCons, err := strconv.ParseInt(miscParts[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid input for accepted connections %q: %v", miscParts[0], err)
	}
	stats.Connections.Accepted = acceptedCons

	handledCons, err := strconv.ParseInt(miscParts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid input for handled connections %q: %v", miscParts[1], err)
	}
	stats.Connections.Handled = handledCons

	requests, err := strconv.ParseInt(miscParts[2], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid input for requests %q: %v", miscParts[2], err)
	}
	stats.Requests = requests

	consParts := strings.Split(strings.TrimSpace(parts[3]), " ")
	if len(consParts) != 6 {
		return fmt.Errorf("invalid input for connections %q", parts[3])
	}

	readingCons, err := strconv.ParseInt(consParts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid input for reading connections %q: %v", consParts[1], err)
	}
	stats.Connections.Reading = readingCons

	writingCons, err := strconv.ParseInt(consParts[3], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid input for writing connections %q: %v", consParts[3], err)
	}
	stats.Connections.Writing = writingCons

	waitingCons, err := strconv.ParseInt(consParts[5], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid input for waiting connections %q: %v", consParts[5], err)
	}
	stats.Connections.Waiting = waitingCons

	return nil
}
