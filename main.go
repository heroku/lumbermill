package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	influx "github.com/influxdb/influxdb-go"
	metrics "github.com/rcrowley/go-metrics"
	librato "github.com/rcrowley/go-metrics/librato"
)

type ShutdownChan chan struct{}

const (
	PointChannelCapacity = 500000
	HashRingReplication  = 46
	PostersPerHost       = 6
)

var (
	connectionCloser = make(chan struct{})
	Debug            = os.Getenv("DEBUG") == "true"

	User     = os.Getenv("USER")
	Password = os.Getenv("PASSWORD")
)

func (s ShutdownChan) Signal() {
	s <- struct{}{}
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
func createClients(hostlist string, skipVerify bool) []influx.ClientConfig {
	clients := make([]influx.ClientConfig, 0)
	for _, host := range strings.Split(hostlist, ",") {
		host = strings.Trim(host, "\t ")
		if host != "" {
			clients = append(clients, createInfluxDBClient(host, skipVerify))
		}
	}
	return clients
}

// Creates destinations and attaches them to posters, which deliver to InfluxDB
func createMessageRoutes(hostlist string, skipVerify bool) (*HashRing, []*Destination, *sync.WaitGroup) {
	posterGroup := new(sync.WaitGroup)
	hashRing := NewHashRing(HashRingReplication, nil)
	destinations := make([]*Destination, 0)

	influxClients := createClients(hostlist, skipVerify)
	if len(influxClients) == 0 {
		//No backends, so blackhole things
		destination := NewDestination("null", PointChannelCapacity)
		hashRing.Add(destination)
		destinations = append(destinations, destination)
		poster := NewNullPoster(destination)
		go poster.Run()
	} else {
		for _, client := range influxClients {
			name := client.Host
			destination := NewDestination(name, PointChannelCapacity)
			hashRing.Add(destination)
			destinations = append(destinations, destination)
			for p := 0; p < PostersPerHost; p++ {
				poster := NewPoster(client, name, destination, posterGroup)
				go poster.Run()
			}
		}
	}

	return hashRing, destinations, posterGroup
}

func awaitSignals(ss ...Signaler) {
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("Got signal: %q", sig)
	for _, s := range ss {
		s.Signal()
	}
}

func awaitShutdown(shutdownChan ShutdownChan, server *LumbermillServer, posterGroup *sync.WaitGroup) {
	<-shutdownChan
	log.Printf("waiting for inflight requests to finish.")
	server.Wait()
	posterGroup.Wait()
	log.Printf("Shutdown complete.")
}

func main() {
	hashRing, destinations, posterGroup := createMessageRoutes(os.Getenv("INFLUXDB_HOSTS"), os.Getenv("INFLUXDB_SKIP_VERIFY") == "true")

	if os.Getenv("LIBRATO_TOKEN") != "" {
		go librato.Librato(
			metrics.DefaultRegistry,
			20*time.Second,
			os.Getenv("LIBRATO_OWNER"),
			os.Getenv("LIBRATO_TOKEN"),
			os.Getenv("LIBRATO_SOURCE"),
			[]float64{0.50, 0.95, 0.99},
			time.Millisecond,
			)
	} else if os.Getenv("DEBUG") == "true" {
		go metrics.Log(metrics.DefaultRegistry, 20e9, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))
	}

	shutdownChan := make(ShutdownChan)
	server := NewLumbermillServer(&http.Server{Addr: ":" + os.Getenv("PORT")}, hashRing)

	log.Printf("Starting up")
	go server.Run(5 * time.Minute)

	signalers := make([]Signaler, 0)
	signalers = append(signalers, server)
	signalers = append(signalers, shutdownChan)
	for _, sig := range destinations {
		signalers = append(signalers, sig)
	}

	go awaitSignals(signalers...)
	awaitShutdown(shutdownChan, server, posterGroup)
}
