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
var influxDbSeriesCheckQueries = []string{
	"select * from MaxMean1mLoad.10m.dyno.dyno.load.%s limit 1",
	"select * from MaxMeanRssSwapMemory.10m.dyno.mem.%s limit 1",
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

	healthCheckClientsLock *sync.Mutex
	healthCheckClients     map[string]*influx.Client
}

func NewLumbermillServer(server *http.Server, ath auth.Authenticater, hashRing *HashRing) *LumbermillServer {
	s := &LumbermillServer{
		connectionCloser:       make(chan struct{}),
		shutdownChan:           make(chan struct{}),
		http:                   server,
		hashRing:               hashRing,
		credStore:              make(map[string]string),
		tokenLock:              new(int32),
		recentTokensLock:       new(sync.RWMutex),
		recentTokens:           make(map[string]string),
		healthCheckClientsLock: new(sync.Mutex),
		healthCheckClients:     make(map[string]*influx.Client),
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

// Serves a 200 OK, unless shutdown has been requested.
// Shutting down serves a 503 since that's how ELBs implement connection draining.
func (s *LumbermillServer) serveHealth(w http.ResponseWriter, r *http.Request) {
	if s.isShuttingDown {
		http.Error(w, "Shutting Down", 503)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *LumbermillServer) getHealthCheckClient(host string) (*influx.Client, error) {
	s.healthCheckClientsLock.Lock()
	defer s.healthCheckClientsLock.Unlock()

	if client, exists := s.healthCheckClients[host]; !exists {
		clientConfig := createInfluxDBClient(host, os.Getenv("INFLUXDB_SKIP_VERIFY") == "true")
		client, err := influx.NewClient(&clientConfig)
		if err != nil {
			log.Printf("err=%q at=getHealthCheckClient host=%q", err, host)
			return nil, err
		} else {
			s.healthCheckClients[host] = client
			return client, nil
		}
	} else {
		return client, nil
	}
}

func (s *LumbermillServer) checkRecentTokens() map[string]int {
	errantHosts := make(map[string]int)

	s.recentTokensLock.RLock()
	tokenMap := make(map[string]string)
	for name, token := range s.recentTokens {
		tokenMap[name] = token
	}
	s.recentTokensLock.RUnlock()

	for name, token := range tokenMap {
		client, err := s.getHealthCheckClient(name)
		if err != nil {
			errantHosts[name]++
			continue
		}

		// Query the last point for the token across all queries and ensure it's been published to in the last XXX minutes.
		for _, qfmt := range influxDbSeriesCheckQueries {
			query := fmt.Sprintf(qfmt, token)

			results, err := client.Query(query, influx.Second)
			if err != nil || len(results) == 0 {
				log.Printf("at=influxdb-health err=%q result_length=%d host=%q query=%q", err, len(results), name, query)
				errantHosts[name]++
			} else {
				t, ok := results[0].Points[0][0].(float64)
				if !ok {
					errantHosts[name]++
					log.Printf("at=influxdb-health err=\"time column was not a number\" host=%q query=%q", name, query)
					continue
				}

				ts := time.Unix(int64(t), int64(0)).UTC()
				now := time.Now().UTC()
				if now.Sub(ts) > influxDbStaleTimeout {
					errantHosts[name]++
					log.Printf("at=influxdb-health err=\"stale data\" host=%q ts=%q now=%q query=%q", name, ts, now, query)
				}
			}
		}
	}

	return errantHosts
}

func (s *LumbermillServer) serveInfluxDBHealth(w http.ResponseWriter, r *http.Request) {
	errantHosts := s.checkRecentTokens()

	if len(errantHosts) > 0 {
		log.Printf("at=influxdb-health err=\"non-zero error count during health check\" count=%d", len(errantHosts))
		w.WriteHeader(http.StatusServiceUnavailable)
		for host, count := range errantHosts {
			w.Write([]byte(fmt.Sprintf("host=%q query_errors=%d, ", host, count)))
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *LumbermillServer) awaitShutdown() {
	<-s.shutdownChan
	log.Printf("Shutting down.")
	s.isShuttingDown = true
}
