package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	lpxgen "github.com/apg/lpxgen"
	metrics "github.com/rcrowley/go-metrics"
)

func TestLumbermillDrain(t *testing.T) {
	sendBatchCount := int64(100)
	sendPointPerBatchCount := int64(10)

	influxdb := setupInfluxDBTestServer(&sleepyHandler{2 * time.Second})
	influxHost := strings.TrimPrefix(influxdb.URL, "https://")

	// snapshot old values
	routerErrorsBefore := routerErrorLinesCounter.Count()
	routerLinesBefore := routerLinesCounter.Count()
	batchBefore := batchCounter.Count()
	pointSuccessBefore := int64(0)

	deliverySizeHistogram.Clear()

	// Get the before count, in case it was registered already
	if psc := metrics.DefaultRegistry.Get("lumbermill.poster.deliver.points." + influxHost); psc != nil {
		pointSuccessBefore = psc.(metrics.Counter).Count()
	}

	defer func() {
		routerErrors := routerErrorLinesCounter.Count() - routerErrorsBefore
		routerLines := routerLinesCounter.Count() - routerLinesBefore
		batches := batchCounter.Count() - batchBefore

		totalExpectedPoints := sendBatchCount * sendPointPerBatchCount

		// This is a bit wonky, but we don't have a *total* points delivered metric.
		// We can determine how many deliveries happened. Empircally, this is 1 for this
		// test. We try to ship 1000 points, so check that 1000 were shipped in that 1
		// delivery.
		psc := metrics.DefaultRegistry.Get("lumbermill.poster.deliver.points." + influxHost)
		if psc != nil {
			pointSuccess := psc.(metrics.Counter).Count() - pointSuccessBefore
			if pointSuccess == 0 {
				t.Errorf("Expected at least one delivery")
			}

			if pointSuccess == 1 && deliverySizeHistogram.Max() != totalExpectedPoints {
				t.Errorf("1 delivery happened, but not all points were published in that delivery. %d/%d", deliverySizeHistogram.Max(), totalExpectedPoints)
			}
		} else {
			t.Errorf("No pointSuccessBefore counter registered")
		}

		if routerErrors > 0 {
			t.Errorf("Some router errors were reported during the test: %d errors", routerErrors)
		}
		if routerLines == 0 {
			t.Errorf("No router lines processed")
		}
		if batches != sendBatchCount {
			t.Errorf("%d lost batches not accounted for", sendBatchCount-batches)
		}
	}()

	lumbermill, testServer, destinations, waitGroup := setupLumbermillTestServer(influxHost, "user:pass")
	shutdownChan := make(ShutdownChan)

	defer func() {
		influxdb.Close()
		testServer.Close()
	}()

	go lumbermill.awaitShutdown()
	go func() {
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		gen := lpxgen.NewGenerator(int(sendPointPerBatchCount),
			int(sendPointPerBatchCount)+1, lpxgen.Router)
		drainUrl := fmt.Sprintf("%s/drain", testServer.URL)

		for i := 0; i < int(sendBatchCount); i++ {
			if _, err := client.Do(gen.Generate(drainUrl)); err != nil {
				t.Errorf("Got an error during client.Do: %q", err)
			}
		}

		// Shutdown by calling Close() on both shutdownChan and lumbermill
		shutdownChan.Close()
		lumbermill.Close()
		for _, d := range destinations {
			d.Close()
		}
	}()

	awaitShutdown(shutdownChan, lumbermill, waitGroup)
}
