package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	influx "github.com/influxdb/influxdb-go"
	metrics "github.com/rcrowley/go-metrics"
)

var influxDeliverySizeHistogram = metrics.GetOrRegisterHistogram("lumbermill.poster.deliver.sizes", metrics.DefaultRegistry, metrics.NewUniformSample(100))

type InfluxDBPoster struct {
	destination          *Destination
	name                 string
	influxClient         *influx.Client
	pointsSuccessCounter metrics.Counter
	pointsSuccessTime    metrics.Timer
	pointsFailureCounter metrics.Counter
	pointsFailureTime    metrics.Timer
	waitGroup            *sync.WaitGroup
}

func NewInfluxDBPoster(clientConfig influx.ClientConfig, name string, destination *Destination, waitGroup *sync.WaitGroup) Poster {
	influxClient, err := influx.NewClient(&clientConfig)

	if err != nil {
		panic(err)
	}

	return &InfluxDBPoster{
		destination:          destination,
		name:                 name,
		influxClient:         influxClient,
		pointsSuccessCounter: metrics.GetOrRegisterCounter("lumbermill.poster.deliver.points."+name, metrics.DefaultRegistry),
		pointsSuccessTime:    metrics.GetOrRegisterTimer("lumbermill.poster.success.time."+name, metrics.DefaultRegistry),
		pointsFailureCounter: metrics.GetOrRegisterCounter("lumbermill.poster.error.points."+name, metrics.DefaultRegistry),
		pointsFailureTime:    metrics.GetOrRegisterTimer("lumbermill.poster.error.time."+name, metrics.DefaultRegistry),
		waitGroup:            waitGroup,
	}
}

func createInfluxDBClient(host string, skipVerify bool) influx.ClientConfig {
	return influx.ClientConfig{
		Host:     host,                       //"influxor.ssl.edward.herokudev.com:8086",
		Username: os.Getenv("INFLUXDB_USER"), //"test",
		Password: os.Getenv("INFLUXDB_PWD"),  //"tester",
		Database: os.Getenv("INFLUXDB_NAME"), //"ingress",
		IsSecure: true,
		HttpClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
				ResponseHeaderTimeout: 5 * time.Second,
				Dial: func(network, address string) (net.Conn, error) {
					return net.DialTimeout(network, address, 5*time.Second)
				},
			},
			Timeout: 10 * time.Second,
		},
	}
}

// Creates clients which deliver to InfluxDB
func createInfluxDBClients(hostlist string, skipVerify bool) []influx.ClientConfig {
	clients := make([]influx.ClientConfig, 0)
	for _, host := range strings.Split(hostlist, ",") {
		host = strings.Trim(host, "\t ")
		if host != "" {
			clients = append(clients, createInfluxDBClient(host, skipVerify))
		}
	}
	return clients
}

func makeInfluxDBSeries(p Point) *influx.Series {
	series := &influx.Series{Points: make([][]interface{}, 0)}
	series.Name = p.SeriesName()
	series.Columns = seriesColumns[p.Type]
	return series
}

func (p *InfluxDBPoster) Run() {
	var last bool
	var delivery map[string]*influx.Series

	p.waitGroup.Add(1)
	timeout := time.NewTicker(time.Second)
	defer func() { timeout.Stop() }()
	defer p.waitGroup.Done()

	for !last {
		delivery, last = p.nextDelivery(timeout)
		p.deliver(delivery)
	}
}

func (p *InfluxDBPoster) nextDelivery(timeout *time.Ticker) (delivery map[string]*influx.Series, last bool) {
	delivery = make(map[string]*influx.Series)
	for {
		select {
		case point, open := <-p.destination.points:
			if open {
				seriesName := point.SeriesName()
				series, found := delivery[seriesName]
				if !found {
					series = makeInfluxDBSeries(point)
				}
				series.Points = append(series.Points, point.Points)
				delivery[seriesName] = series
			} else {
				return delivery, true
			}
		case <-timeout.C:
			return delivery, false
		}
	}
}

func (p *InfluxDBPoster) deliver(allSeries map[string]*influx.Series) {
	pointCount := 0
	seriesGroup := make([]*influx.Series, 0, len(allSeries))

	for _, s := range allSeries {
		pointCount += len(s.Points)
		seriesGroup = append(seriesGroup, s)
	}

	if pointCount == 0 {
		return
	}

	start := time.Now()
	err := p.influxClient.WriteSeriesWithTimePrecision(seriesGroup, influx.Microsecond)

	if err != nil {
		// TODO: Ugh. These could be timeout errors, or an internal error.
		//       Should probably attempt to figure out which...
		p.pointsFailureCounter.Inc(1)
		p.pointsFailureTime.UpdateSince(start)
		log.Printf("Error posting points: %s\n", err)
	} else {
		p.pointsSuccessCounter.Inc(1)
		p.pointsSuccessTime.UpdateSince(start)
		influxDeliverySizeHistogram.Update(int64(pointCount))
	}
}
