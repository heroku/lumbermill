package main

import (
	"crypto/tls"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	posters      = make([]*Poster, 0)
	destinations = make([]*Destination, 0)

	hashRing = NewHashRing(HashRingReplication, func(data []byte) uint32 {
		a := fnv.New32a()
		a.Write(data)
		return a.Sum32()
	})

	Debug = os.Getenv("DEBUG") == "true"

	User     = os.Getenv("USER")
	Password = os.Getenv("PASSWORD")
)

func (s ShutdownChan) Signal() {
	s <- struct{}{}
}

func createInfluxDBClient(host string) influx.ClientConfig {
	return influx.ClientConfig{
		Host:     host,                       //"influxor.ssl.edward.herokudev.com:8086",
		Username: os.Getenv("INFLUXDB_USER"), //"test",
		Password: os.Getenv("INFLUXDB_PWD"),  //"tester",
		Database: os.Getenv("INFLUXDB_NAME"), //"ingress",
		IsSecure: true,
		HttpClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: os.Getenv("INFLUXDB_SKIP_VERIFY") == "true"},
				ResponseHeaderTimeout: 5 * time.Second,
				Dial: func(network, address string) (net.Conn, error) {
					return net.DialTimeout(network, address, 5*time.Second)
				},
			},
			Timeout: 10 * time.Second,
		},
	}
}

func createClients(hostlist string) []influx.ClientConfig {
	clients := make([]influx.ClientConfig, 0)
	for _, host := range strings.Split(hostlist, ",") {
		host = strings.Trim(host, "\t ")
		if host != "" {
			clients = append(clients, createInfluxDBClient(host))
		}
	}
	return clients
}


func awaitShutdownSignals(ss ... Signaler) {
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("Got signal: %q", sig)
	for _, s := range ss {
		s.Signal()
	}
}


func main() {
	influxClients := createClients(os.Getenv("INFLUXDB_HOSTS"))
	if len(influxClients) == 0 {
		//No backends, so blackhole things
		destination := NewDestination("null", PointChannelCapacity)
		destinations = append(destinations, destination)
		poster := NewNullPoster(destination)
		go poster.Run()
	} else {
		for _, client := range influxClients {
			name := client.Host
			destination := NewDestination(name, PointChannelCapacity)
			destinations = append(destinations, destination)

			for p := 0; p < PostersPerHost; p++ {
				poster := NewPoster(client, name, destination)
				posters = append(posters, poster)
				go poster.Run()
			}
		}
	}

	hashRing.Add(destinations...)

	go librato.Librato(
		metrics.DefaultRegistry,
		20*time.Second,
		os.Getenv("LIBRATO_OWNER"),
		os.Getenv("LIBRATO_TOKEN"),
		os.Getenv("LIBRATO_SOURCE"),
		[]float64{0.50, 0.95, 0.99},
		time.Millisecond,
	)

	shutdownChan := make(ShutdownChan)
	server := NewHttpServer()

	go server.Run(os.Getenv("PORT"), 5 * time.Minute)
	go awaitShutdownSignals(server, shutdownChan)

	log.Printf("Starting up")
	<- shutdownChan
	log.Printf("waiting for inflight requests to finish.")
	server.Wait()
	log.Printf("Shutdown complete.")
}
