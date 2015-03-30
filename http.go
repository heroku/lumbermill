package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	auth "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/heroku/authenticater"
	influx "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/influxdb/influxdb-go"
)

var influxDbStaleTimeout = 24 * time.Minute // Would be nice to make this smaller, but it lags due to continuous queries.
var influxDbSeriesCheckPrefixes = []string{
	"MaxMean1mLoad.10m.dyno.dyno.load.",
	"MaxMeanRssSwapMemory.10m.dyno.mem.",
}

type LumbermillServer struct {
	sync.WaitGroup
	connectionCloser chan struct{}
	hashRing         *HashRing
	http             *http.Server
	shutdownChan     ShutdownChan
	isShuttingDown   bool
	credStore        map[string]string

	// scheduler based sampling lock for writing to recentTokens
	tokenLock        *int32
	recentTokensLock *sync.RWMutex
	recentTokens     map[string]string
}

func NewLumbermillServer(server *http.Server, ath auth.Authenticater, hashRing *HashRing) *LumbermillServer {
	s := &LumbermillServer{
		connectionCloser: make(chan struct{}),
		shutdownChan:     make(chan struct{}),
		http:             server,
		hashRing:         hashRing,
		credStore:        make(map[string]string),
		tokenLock:        new(int32),
		recentTokensLock: new(sync.RWMutex),
		recentTokens:     make(map[string]string),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/drain", auth.WrapAuth(ath,
		func(w http.ResponseWriter, r *http.Request) {
			s.serveDrain(w, r)
			s.recycleConnection(w)
		}))

	mux.HandleFunc("/health", s.serveHealth)
	mux.HandleFunc("/health/influxdb", s.serveInfluxDBHealth)
	mux.HandleFunc("/target/", auth.WrapAuth(ath, s.serveTarget))

	s.http.Handler = mux

	return s
}

func (s *LumbermillServer) Close() error {
	s.shutdownChan <- struct{}{}
	return nil
}

func (s *LumbermillServer) scheduleConnectionRecycling(after time.Duration) {
	for !s.isShuttingDown {
		time.Sleep(after)
		s.connectionCloser <- struct{}{}
	}
}

func (s *LumbermillServer) recycleConnection(w http.ResponseWriter) {
	select {
	case <-s.connectionCloser:
		w.Header().Set("Connection", "close")
	default:
		if s.isShuttingDown {
			w.Header().Set("Connection", "close")
		}
	}
}

func (s *LumbermillServer) Run(connRecycle time.Duration) {
	go s.awaitShutdown()
	go s.scheduleConnectionRecycling(connRecycle)

	if err := s.http.ListenAndServe(); err != nil {
		log.Fatalln("Unable to start HTTP server: ", err)
	}
}

func (s *LumbermillServer) serveHealth(w http.ResponseWriter, r *http.Request) {
	if s.isShuttingDown {
		http.Error(w, "Shutting Down", 503)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *LumbermillServer) serveInfluxDBHealth(w http.ResponseWriter, r *http.Request) {
	errorCount := 0

	// Copy the map so we can unlock.
	s.recentTokensLock.RLock()
	tokenMap := make(map[string]string)
	for name, token := range s.recentTokens {
		tokenMap[name] = token
	}
	s.recentTokensLock.RUnlock()

	for name, token := range tokenMap {
		// Ugh. For now. Should reuse connections, but tied up in Posters, and abstraction.
		clientConfig := createInfluxDBClient(name, os.Getenv("INFLUXDB_SKIP_VERIFY") == "true")
		client, _ := influx.NewClient(&clientConfig)

		// Query the last point for the token and ensure it's been published to in the last XXX minutes.
		for _, prefix := range influxDbSeriesCheckPrefixes {
			series := prefix + token
			query := fmt.Sprintf("select * from %s limit 1", series)

			results, err := client.Query(query, influx.Second)
			if err != nil || len(results) == 0 {
				errorCount++
				log.Printf("at=influxdb-health err=%q result_length=%d host=%q token=%q", err, len(results), name, token)
			} else {
				t, ok := results[0].Points[0][0].(float64)
				if !ok {
					errorCount++
					log.Printf("at=influxdb-health err=\"time column was not an int\" host=%q token=%q", name, token)
					continue
				}

				ts := time.Unix(int64(t), int64(0)).UTC()
				now := time.Now().UTC()
				if now.Sub(ts) > influxDbStaleTimeout {
					errorCount++
					log.Printf("at=influxdb-health err=\"stale data\" host=%q ts=%q now=%q token=%q", name, ts, now, token)
				}
			}
		}
	}

	if errorCount > 0 {
		log.Printf("at=influxdb-health err=\"non-zero error count during health check\" count=%d", errorCount)
		http.Error(w, "Failed Health Check", 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *LumbermillServer) awaitShutdown() {
	<-s.shutdownChan
	log.Printf("Shutting down.")
	s.isShuttingDown = true
}
