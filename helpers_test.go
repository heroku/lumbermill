package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	auth "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/heroku/authenticater"
)

const defaultTestClientTimeout = 1 * time.Second

func newSleepyHandler(amt time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(amt)
		w.WriteHeader(http.StatusOK)
	}
}

func newFixedResultHandler(contentType, result string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", contentType)
		w.Write([]byte(result))
	}
}

func newFixedStatusHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(status)
	}
}

func newTestClientFunc() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: defaultTestClientTimeout,
	}
}

func setupInfluxDBTestServer(handler http.Handler) *httptest.Server {
	if handler == nil {
		mux := http.NewServeMux()
		mux.HandleFunc("/db", func(w http.ResponseWriter, req *http.Request) {
			log.Printf("INFLUXDB: Got a request\n")
			w.WriteHeader(http.StatusOK)
		})
		handler = mux
	}
	return httptest.NewTLSServer(handler)
}

func setupLumbermillTestServer(influxHosts, creds string) (*server, *httptest.Server, []*destination, *sync.WaitGroup) {
	hashRing, destinations, waitGroup := createMessageRoutes(influxHosts, true)
	testServer := httptest.NewServer(nil)
	lumbermill := newServer(testServer.Config, auth.AnyOrNoAuth{}, hashRing)
	return lumbermill, testServer, destinations, waitGroup
}

func splitURL(url string) (string, int) {
	bits := strings.Split(url, ":")
	port, _ := strconv.ParseInt(bits[1], 10, 16)
	return bits[0], int(port)
}

func extractHostPort(url string) string {
	urlBits := strings.Split(url, "//")
	if len(urlBits) > 1 {
		return urlBits[1]
	}
	panic("Unable to parse URL into host:port")
}
