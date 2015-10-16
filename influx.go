package main

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	influx "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/influxdb/influxdb-go"
)

type clientFunc func() *http.Client

func createInfluxDBClient(host string, f clientFunc) influx.ClientConfig {
	return influx.ClientConfig{
		Host:       host,                       //"influxor.ssl.edward.herokudev.com:8086",
		Username:   os.Getenv("INFLUXDB_USER"), //"test",
		Password:   os.Getenv("INFLUXDB_PWD"),  //"tester",
		Database:   os.Getenv("INFLUXDB_NAME"), //"ingress",
		IsSecure:   os.Getenv("INFLUXDB_INSECURE") != "true",
		HttpClient: f(),
	}
}

// Creates clients which deliver to InfluxDB
func createClients(hostlist string, f clientFunc) []influx.ClientConfig {
	var clients []influx.ClientConfig

	for _, host := range strings.Split(hostlist, ",") {
		host = strings.Trim(host, "\t ")
		if host != "" {
			clients = append(clients, createInfluxDBClient(host, f))
		}
	}
	return clients
}

// Creates destinations and attaches them to posters, which deliver to InfluxDB
func createMessageRoutes(hostlist string, f clientFunc) (*hashRing, []*destination, *sync.WaitGroup) {
	var destinations []*destination
	posterGroup := new(sync.WaitGroup)
	hashRing := newHashRing(hashRingReplication, nil)

	influxClients := createClients(hostlist, f)
	if len(influxClients) == 0 {
		//No backends, so blackhole things
		destination := newDestination("null", pointChannelCapacity)
		hashRing.Add(destination)
		destinations = append(destinations, destination)
		poster := newNullPoster(destination)
		go poster.Run()
	} else {
		for _, client := range influxClients {
			name := client.Host
			destination := newDestination(name, pointChannelCapacity)
			hashRing.Add(destination)
			destinations = append(destinations, destination)
			for p := 0; p < postersPerHost; p++ {
				poster := newPoster(client, name, destination, posterGroup)
				posterGroup.Add(1)
				go func() {
					poster.Run()
					posterGroup.Done()
				}()
			}
		}
	}

	return hashRing, destinations, posterGroup
}

func newClientFunc() *http.Client {
	if os.Getenv("INFLUXDB_INSECURE") == "true" {
		return &http.Client{Timeout: defaultClientTimeout}
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: os.Getenv("INFLUXDB_SKIP_VERIFY") == "true"},
		},
		Timeout: defaultClientTimeout,
	}
}

type shutdownChan chan struct{}

const (
	defaultClientTimeout = 20 * time.Second
	pointChannelCapacity = 500000
	hashRingReplication  = 46
	postersPerHost       = 6
)

var (
	connectionCloser = make(chan struct{})
	debug            = os.Getenv("DEBUG") == "true"
)

func (s shutdownChan) Close() error {
	s <- struct{}{}
	return nil
}

func awaitSignals(ss ...io.Closer) {
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("Got signal: %q", sig)
	for _, s := range ss {
		s.Close()
	}
}

func awaitShutdown(shutdownChan shutdownChan, server *server, posterGroup *sync.WaitGroup) {
	<-shutdownChan
	log.Printf("waiting for inflight requests to finish.")
	server.Wait()
	posterGroup.Wait()
	log.Printf("Shutdown complete.")
}
