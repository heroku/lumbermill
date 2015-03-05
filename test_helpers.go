package main

import (
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

type sleepyHandler struct {
	Amt time.Duration
}

func (s *sleepyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	time.Sleep(s.Amt)
	w.WriteHeader(http.StatusOK)
}

func setupInfluxDBTestServer(handler http.Handler) *httptest.Server {
	if handler == nil {
		handler := http.NewServeMux()
		handler.HandleFunc("/db", func(w http.ResponseWriter, req *http.Request) {
			log.Printf("INFLUXDB: Got a request\n")
			w.WriteHeader(http.StatusOK)
		})
	}

	return httptest.NewTLSServer(handler)
}

func setupLumbermillTestServer(influxHosts, creds string) (*LumbermillServer, *httptest.Server, []*Destination, *sync.WaitGroup) {
	hashRing, destinations, waitGroup := createMessageRoutes(influxHosts, true)
	testServer := httptest.NewServer(nil)
	lumbermill := NewLumbermillServer(testServer.Config, hashRing, creds)
	return lumbermill, testServer, destinations, waitGroup
}

func splitURL(url string) (string, int) {
	bits := strings.Split(url, ":")
	port, _ := strconv.ParseInt(bits[1], 10, 16)
	return bits[0], int(port)
}
