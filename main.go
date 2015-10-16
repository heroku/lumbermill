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
)

func main() {
	hashRing, destinations, posterGroup := createMessageRoutes(os.Getenv("INFLUXDB_HOSTS"), newClientFunc)

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

	basicAuther, err := auth.NewBasicAuthFromString(os.Getenv("CRED_STORE"))
	if err != nil {
		log.Fatalf("Unable to parse credentials from CRED_STORE=%q: err=%q", os.Getenv("CRED_STORE"), err)
	}

	shutdownChan := make(shutdownChan)
	server := newServer(&http.Server{Addr: ":" + os.Getenv("PORT")}, basicAuther, hashRing)

	log.Printf("Starting up")
	go server.Run(5 * time.Minute)

	var closers []io.Closer
	closers = append(closers, server)
	closers = append(closers, shutdownChan)
	for _, cls := range destinations {
		closers = append(closers, cls)
	}

	go awaitSignals(closers...)
	awaitShutdown(shutdownChan, server, posterGroup)
}
