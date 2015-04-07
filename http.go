package main

import (
	"fmt"
	"log"
	"net/http"
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

var healthCheckClientsLock = new(sync.Mutex)
var healthCheckClients = make(map[string]*influx.Client)

type server struct {
	sync.WaitGroup
	connectionCloser chan struct{}
	hashRing         *hashRing
	http             *http.Server
	shutdownChan     shutdownChan
	isShuttingDown   bool
	credStore        map[string]string

	// scheduler based sampling lock for writing to recentTokens
	tokenLock        *int32
	recentTokensLock *sync.RWMutex
	recentTokens     map[string]string
}

func newServer(httpServer *http.Server, ath auth.Authenticater, hashRing *hashRing) *server {
	s := &server{
		connectionCloser: make(chan struct{}),
		shutdownChan:     make(chan struct{}),
		http:             httpServer,
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

func (s *server) Close() error {
	s.shutdownChan <- struct{}{}
	return nil
}

func (s *server) scheduleConnectionRecycling(after time.Duration) {
	for !s.isShuttingDown {
		time.Sleep(after)
		s.connectionCloser <- struct{}{}
	}
}

func (s *server) recycleConnection(w http.ResponseWriter) {
	select {
	case <-s.connectionCloser:
		w.Header().Set("Connection", "close")
	default:
		if s.isShuttingDown {
			w.Header().Set("Connection", "close")
		}
	}
}

func (s *server) Run(connRecycle time.Duration) {
	go s.awaitShutdown()
	go s.scheduleConnectionRecycling(connRecycle)

	if err := s.http.ListenAndServe(); err != nil {
		log.Fatalln("Unable to start HTTP server: ", err)
	}
}

// Serves a 200 OK, unless shutdown has been requested.
// Shutting down serves a 503 since that's how ELBs implement connection draining.
func (s *server) serveHealth(w http.ResponseWriter, r *http.Request) {
	if s.isShuttingDown {
		http.Error(w, "Shutting Down", 503)
	}

	w.WriteHeader(http.StatusOK)
}

func getHealthCheckClient(host string, f clientFunc) (*influx.Client, error) {
	healthCheckClientsLock.Lock()
	defer healthCheckClientsLock.Unlock()

	if client, exists := healthCheckClients[host]; !exists {
		clientConfig := createInfluxDBClient(host, f)
		client, err := influx.NewClient(&clientConfig)
		if err != nil {
			log.Printf("err=%q at=getHealthCheckClient host=%q", err, host)
			return nil, err
		}

		healthCheckClients[host] = client
		return client, nil
	}

	return client, nil
}

func checkRecentToken(client *influx.Client, token, host string, errors chan error) {
	for _, qfmt := range influxDbSeriesCheckQueries {
		query := fmt.Sprintf(qfmt, token)
		results, err := client.Query(query, influx.Second)
		if err != nil || len(results) == 0 {
			errors <- fmt.Errorf("at=influxdb-health err=%q result_length=%d host=%q query=%q", err, len(results), host, query)
			continue
		}

		t, ok := results[0].Points[0][0].(float64)
		if !ok {
			errors <- fmt.Errorf("at=influxdb-health err=\"time column was not a number\" host=%q query=%q", host, query)
			continue
		}

		ts := time.Unix(int64(t), int64(0)).UTC()
		now := time.Now().UTC()
		if now.Sub(ts) > influxDbStaleTimeout {
			errors <- fmt.Errorf("at=influxdb-health err=\"stale data\" host=%q ts=%q now=%q query=%q", host, ts, now, query)
		}
	}
}

func (s *server) checkRecentTokens() []error {
	var errSlice []error

	wg := new(sync.WaitGroup)

	s.recentTokensLock.RLock()
	tokenMap := make(map[string]string)
	for host, token := range s.recentTokens {
		tokenMap[host] = token
	}
	s.recentTokensLock.RUnlock()

	errors := make(chan error, len(tokenMap)*len(influxDbSeriesCheckQueries))

	for host, token := range tokenMap {
		wg.Add(1)
		go func(token, host string) {
			client, err := getHealthCheckClient(host, nil)
			if err != nil {
				return
			}
			checkRecentToken(client, token, host, errors)
			wg.Done()
		}(token, host)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		errSlice = append(errSlice, err)
	}

	return errSlice
}

func (s *server) serveInfluxDBHealth(w http.ResponseWriter, r *http.Request) {
	errors := s.checkRecentTokens()

	if len(errors) > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		for _, err := range errors {
			w.Write([]byte(err.Error() + "\n"))
			log.Println(err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) awaitShutdown() {
	<-s.shutdownChan
	log.Printf("Shutting down.")
	s.isShuttingDown = true
}
