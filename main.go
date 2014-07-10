package main

import (
	"crypto/tls"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	librato "github.com/rcrowley/go-metrics/librato"

	"github.com/heroku/slog"
	influx "github.com/influxdb/influxdb-go"
)

const (
	PointChannelCapacity = 100000
	HashRingReplication  = 46 // TODO: Needs to be determined
	PostersPerHost       = 6
)

const (
	Router = iota
	EventsRouter
	DynoMem
	DynoLoad
	EventsDyno
	numSeries
)

var (
	connectionCloser = make(chan struct{})

	posters    = make([]*Poster, 0)
	chanGroups = make([]*ChanGroup, 0)

	seriesNames = []string{"router", "events.router", "dyno.mem", "dyno.load", "events.dyno"}

	seriesColumns = [][]string{
		[]string{"time", "id", "status", "service"}, // Router
		[]string{"time", "id", "code"},              // EventsRouter
		[]string{"time", "id", "source", "memory_cache", "memory_pgpgin", "memory_pgpgout", "memory_rss", "memory_swap", "memory_total", "dynoType"}, // DynoMem
		[]string{"time", "id", "source", "load_avg_1m", "load_avg_5m", "load_avg_15m", "dynoType"},                                                   // DynoLoad
		[]string{"time", "id", "what", "type", "code", "message", "dynoType"},                                                                        // DynoEvents
	}

	hashRing = NewHashRing(HashRingReplication, func(data []byte) uint32 {
		a := fnv.New32a()
		a.Write(data)
		return a.Sum32()
	})

	Debug = os.Getenv("DEBUG") == "true"

	User     = os.Getenv("USER")
	Password = os.Getenv("PASSWORD")
)

func LogWithContext(ctx slog.Context) {
	ctx.Add("app", "lumbermill")
	log.Println(ctx)
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

// Health Checks, so just say 200 - OK
// TODO: Actual healthcheck
func serveHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func main() {
	port := os.Getenv("PORT")

	influxClients := createClients(os.Getenv("INFLUXDB_HOSTS"))
	if len(influxClients) == 0 {
		//No backends, so blackhole things
		group := NewChanGroup("null", PointChannelCapacity)
		chanGroups = append(chanGroups, group)
		poster := NewNullPoster(group)
		go poster.Run()
	} else {
		for _, client := range influxClients {
			name := client.Host
			group := NewChanGroup(name, PointChannelCapacity)
			chanGroups = append(chanGroups, group)

			for p := 0; p < PostersPerHost; p++ {
				poster := NewPoster(client, name, group)
				posters = append(posters, poster)
				go poster.Run()
			}
		}
	}

	hashRing.Add(chanGroups...)

	go librato.Librato(
		metrics.DefaultRegistry,
		20*time.Second,
		os.Getenv("LIBRATO_OWNER"),
		os.Getenv("LIBRATO_TOKEN"),
		os.Getenv("LIBRATO_SOURCE"),
		[]float64{0.50, 0.95, 0.99},
		time.Millisecond,
	)

	// Every 5 minutes, signal that the connection should be closed
	// This should allow for a slow balancing of connections.
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			connectionCloser <- struct{}{}
		}
	}()

	http.HandleFunc("/drain", serveDrain)
	http.HandleFunc("/health", serveHealth)
	http.HandleFunc("/target/", serveTarget)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
