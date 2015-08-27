package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	auth "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/heroku/authenticater"
)

func TestTargetWithMultipleAuth(t *testing.T) {
	ba := auth.NewBasicAuth()
	ba.AddPrincipal("user1", "pass1")
	ba.AddPrincipal("user2", "pass2")
	server := newServer(&http.Server{}, ba, newHashRing(1, nil))

	recorder := httptest.NewRecorder()

	for i := 1; i <= 2; i++ {
		req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("foo")))
		req.SetBasicAuth(fmt.Sprintf("user%d", i), fmt.Sprintf("pass%d", i))
		if err != nil {
			t.Fatal(err)
		}

		server.serveTarget(recorder, req)

		if recorder.Code == http.StatusForbidden {
			t.Fatal("Provided proper credentials, was forbidden.")
		}
	}
}

func TestTargetWithMultiplePasswords(t *testing.T) {
	ba := auth.NewBasicAuth()
	ba.AddPrincipal("user", "pass1")
	ba.AddPrincipal("user", "pass2")
	server := newServer(&http.Server{}, ba, newHashRing(1, nil))

	recorder := httptest.NewRecorder()

	for i := 1; i <= 2; i++ {
		req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("foo")))
		req.SetBasicAuth("user", fmt.Sprintf("pass%d", i))
		if err != nil {
			t.Fatal(err)
		}

		server.serveTarget(recorder, req)

		if recorder.Code == http.StatusForbidden {
			t.Fatal("Provided proper credentials, was forbidden.")
		}
	}
}

func TestTargetWithoutAuth(t *testing.T) {
	ba := auth.NewBasicAuth()
	ba.AddPrincipal("foo", "foo")
	server := newServer(&http.Server{}, ba, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("")))
	if err != nil {
		t.Fatal(err)
	}

	server.http.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}
}

func TestTargetWithoutId(t *testing.T) {
	//Setup
	ba := auth.NewBasicAuth()
	ba.AddPrincipal("foo", "foo")
	server := newServer(&http.Server{}, ba, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/", bytes.NewReader([]byte("")))
	req.SetBasicAuth("foo", "foo")
	if err != nil {
		t.Fatal(err)
	}

	server.http.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}
}

func TestTargetWithoutRing(t *testing.T) {
	server := newServer(&http.Server{}, auth.AnyOrNoAuth{}, newHashRing(1, nil))

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("")))
	//req.SetBasicAuth("foo", "foo")
	if err != nil {
		t.Fatal(err)
	}

	server.http.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}
}

func TestTarget(t *testing.T) {
	os.Setenv("INFLUXDB_HOSTS", "null")
	hashRing, _, _ := createMessageRoutes("null", newTestClientFunc)
	server := newServer(&http.Server{}, auth.AnyOrNoAuth{}, hashRing)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("")))
	req.SetBasicAuth("foo", "foo")
	if err != nil {
		t.Fatal(err)
	}

	server.http.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}

	body := recorder.Body.String()

	if body != "{ \"host\": \"null\" }" {
		t.Fatal("Wrong Body: ", body)
	}
}
