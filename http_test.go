package main

import "testing"

func TestInfluxDBHealth_HappyPath(t *testing.T) {
	influxDB := setupInfluxDBTestServer(nil)
	defer influxDB.Close()

	errors := make(chan error, 100)

	token := "foo"
	host := extractHostPort(influxDB.URL)

	client, err := getHealthCheckClient(host, true)
	if err != nil {
		t.Fatalf(err.Error())
	}

	checkRecentToken(client, token, host, errors)
	close(errors)

	for err := range errors {
		t.Errorf("Unexpected error err=%q", err)
	}
}

func TestInfluxDBHealth_InvalidResponse(t *testing.T) {
	influxDB := setupInfluxDBTestServer(nil)
	defer influxDB.Close()

	token := "foo"
	host := extractHostPort(influxDB.URL)

	client, err := getHealthCheckClient(host, true)
	if err != nil {
		t.Fatalf(err.Error())
	}

	errors := make(chan error, 100)
	checkRecentToken(client, token, host, errors)
	close(errors)

	for _ = range errors {
		return
	}

	t.Errorf("Expected error not found.")
}
