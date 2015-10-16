package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	auth "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/heroku/authenticater"
	metrics "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/rcrowley/go-metrics"
	librato "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/rcrowley/go-metrics/librato"
	"github.com/heroku/lumbermill/destinations"
)

func main() {
	hashRing, dest, posterGroup := destinations.CreateMessageRoutes(os.Getenv("INFLUXDB_HOSTS"), nil)

	// Report to librato given LIBRATO_* config vars
	setupMetrics()

	basicAuther, err := auth.NewBasicAuthFromString(os.Getenv("CRED_STORE"))
	if err != nil {
		log.Fatalf("Unable to parse credentials from CRED_STORE=%q: err=%q", os.Getenv("CRED_STORE"), err)
	}

	shutdownChan := make(destinations.ShutdownChan)
	server := destinations.NewServer(&http.Server{Addr: ":" + os.Getenv("PORT")}, basicAuther, hashRing)

	log.Printf("Starting up")
	go server.Run(5 * time.Minute)

	var closers []io.Closer
	closers = append(closers, server)
	closers = append(closers, shutdownChan)
	for _, cls := range dest {
		closers = append(closers, cls)
	}

	go destinations.AwaitSignals(closers...)
	destinations.AwaitShutdown(shutdownChan, server, posterGroup)
}

func setupMetrics() {
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
}
