package destinations

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestInfluxDBHealth_HappyPath(t *testing.T) {
	fixedContent := fmt.Sprintf(`[{"name":"Throughput.10m.router.blargh","columns":["time","sequence_number","count","status"],"points":[[%d,2,1,201]]}]`,
		time.Now().Unix())

	influxDB := setupInfluxDBTestServer(newFixedResultHandler("application/json", fixedContent))
	defer influxDB.Close()

	errors := make(chan error, 100)

	token := "foo"
	host := extractHostPort(influxDB.URL)

	client, err := getHealthCheckClient(host, newTestClientFunc)
	if err != nil {
		t.Fatalf(err.Error())
	}

	checkRecentToken(client, token, host, errors)
	close(errors)

	for err := range errors {
		t.Errorf("Unexpected error err=%q", err)
	}
}

var invalidInputHandlers = map[string]http.HandlerFunc{
	"Returned Invalid JSON": newFixedResultHandler("application/json", "input"),

	"Stale Data": newFixedResultHandler("application/json", `[{"name":"Throughput.10m.router.blargh","columns":["time","sequence_number","count","status"],"points":[[1920,2,1,201]]}]`),

	"Timestamp NaN": newFixedResultHandler("application/json", `[{"name":"Throughput.10m.router.blargh","columns":["time","sequence_number","count","status"],"points":[["foo",2,1,201]]}]`),

	"Invalid Content-Type": newFixedResultHandler("text/html", "<html></html>"),

	"Invalid Response": newFixedStatusHandler(400),

	"Client Timeout": newSleepyHandler(2 * defaultTestClientTimeout),
}

func TestInfluxDBHealth_InvalidInputs(t *testing.T) {
	for testName, handler := range invalidInputHandlers {
		influxDB := setupInfluxDBTestServer(handler)
		token := "foo"
		host := extractHostPort(influxDB.URL)

		client, err := getHealthCheckClient(host, newTestClientFunc)
		if err != nil {
			influxDB.Close()
			t.Fatalf("test=%q msg=\"Error while getting health check\" client err=%q", err)
		}

		errors := make(chan error, 100)
		checkRecentToken(client, token, host, errors)
		close(errors)
		influxDB.Close()

		errorCnt := 0
		for err := range errors {
			t.Logf("test=%q expected_error=%q", testName, err)
			errorCnt++
		}

		if errorCnt == 0 {
			t.Errorf("err=\"Expected error(s) not found\" test=%q", testName)
		}
	}
}
