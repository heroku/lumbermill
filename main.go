package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kr/s3/s3util"
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

func (s ShutdownChan) Close() error {
	s <- struct{}{}
	return nil
}

// Creates destinations and attaches them to posters, which deliver to InfluxDB
func createMessageRoutes(hostlist string, skipVerify bool) (*HashRing, []*Destination, *sync.WaitGroup) {
	posterGroup := new(sync.WaitGroup)
	hashRing := NewHashRing(HashRingReplication, nil)
	destinations := make([]*Destination, 0)

	influxClients := createInfluxDBClients(hostlist, skipVerify)
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
				poster := NewInfluxDBPoster(client, name, destination, posterGroup)
				go poster.Run()
			}
		}
	}

	return hashRing, destinations, posterGroup
}

// Creates destinations and attaches them to posters, which deliver to S3
func createS3Routes(baseURL, accessKey, secretKey string) (*HashRing, []*Destination, *sync.WaitGroup) {
	posterGroup := new(sync.WaitGroup)
	hashRing := NewHashRing(HashRingReplication, nil)
	destinations := make([]*Destination, 0)

	config := &s3util.Config{}
	config.AccessKey = accessKey
	config.SecretKey = secretKey

	destination := NewDestination(baseURL, PointChannelCapacity)
	hashRing.Add(destination)
	destinations = append(destinations, destination)
	poster := NewS3Poster(destination, baseURL, config, posterGroup)
	go poster.Run()

	return hashRing, destinations, posterGroup
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

func awaitShutdown(shutdownChan ShutdownChan, server *LumbermillServer, posterGroup *sync.WaitGroup) {
	<-shutdownChan
	log.Printf("waiting for inflight requests to finish.")
	server.Wait()
	posterGroup.Wait()
	log.Printf("Shutdown complete.")
}

func main() {
	var hashRing *HashRing
	var destinations []*Destination
	var posterGroup *sync.WaitGroup

	switch {
	case os.Getenv("INFLUXDB_HOSTS") != "":
		hashRing, destinations, posterGroup = createMessageRoutes(os.Getenv("INFLUXDB_HOSTS"), os.Getenv("INFLUXDB_SKIP_VERIFY") == "true")
	case os.Getenv("S3_BUCKET_URL") != "":
		hashRing, destinations, posterGroup = createS3Routes(os.Getenv("S3_BUCKET_URL"), os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"))
	default:
		// just create a null poster
		hashRing, destinations, posterGroup = createMessageRoutes("", os.Getenv("INFLUXDB_SKIP_VERIFY") == "true")
	}

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

	closers := make([]io.Closer, 0)
	closers = append(closers, server)
	closers = append(closers, shutdownChan)
	for _, cls := range destinations {
		closers = append(closers, cls)
	}

	go awaitSignals(closers...)
	awaitShutdown(shutdownChan, server, posterGroup)
}
