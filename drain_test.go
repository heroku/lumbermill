package main

import (
//	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	lpxgen "github.com/apg/lpxgen"
//	metrics "github.com/rcrowley/go-metrics"
)

type SleepyHandler struct {
	Amt time.Duration
}

func (s *SleepyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	time.Sleep(s.Amt)
	w.WriteHeader(http.StatusOK)
}

func SetupInfluxDB(handler http.Handler) *httptest.Server {
	if handler == nil {
		handler := http.NewServeMux()
		handler.HandleFunc("/db", func(w http.ResponseWriter, req *http.Request) {
			log.Printf("INFLUXDB: Got a request\n")
			w.WriteHeader(http.StatusOK)
		})
	}

	return httptest.NewTLSServer(handler)
}

func SetupLumbermill(influxHosts string) (*LumbermillServer, *httptest.Server, *sync.WaitGroup) {
	hashRing, waitGroup := createMessageRoutes(influxHosts)
	testServer := httptest.NewServer(nil)
	lumbermill := NewLumbermillServer(testServer.Config, hashRing)
	return lumbermill, testServer, waitGroup
}

func splitUrl(url string) (string, int) {
	bits := strings.Split(url, ":")
	port, _ := strconv.ParseInt(bits[1], 10, 16)
	return bits[0], int(port)
}

func TestLumbermillDrain(t *testing.T) {
	// get the metric values currently

	influxdb := SetupInfluxDB(&SleepyHandler{5 * time.Second})

	lumbermill, testServer, waitGroup := SetupLumbermill(influxdb.Config.Addr)
	shutdownChan := make(ShutdownChan)

	go lumbermill.awaitShutdown()

	go func() {
		client := &http.Client{}
		gen := lpxgen.NewGenerator(1, 10, lpxgen.Router)
		drainUrl := fmt.Sprintf("%s/drain", testServer.URL)

		if _, err := client.Do(gen.Generate(drainUrl)); err != nil {
			t.Errorf("Got an error during client.Do: %q", err)
		}

		// Shutdown by calling Signal() on both shutdownChan and lumbermill
		shutdownChan.Signal()
		lumbermill.Signal()
	}()

	awaitShutdown(shutdownChan, lumbermill, waitGroup)

	// close open servers
	influxdb.Close()
	testServer.Close()

	// assert metrics values are now - original == expected
}

