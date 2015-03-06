package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTargetWithMultipleAuth(t *testing.T) {
	server := NewLumbermillServer(&http.Server{}, NewHashRing(1, nil))
	server.AddPrincipal("user1", "pass1")
	server.AddPrincipal("user2", "pass2")

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

func TestTargetWithoutAuth(t *testing.T) {
	server := NewLumbermillServer(&http.Server{}, nil)
	server.AddPrincipal("foo", "foo")

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("")))
	if err != nil {
		t.Fatal(err)
	}

	server.http.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}
}

func TestTargetWithoutId(t *testing.T) {
	//Setup
	server := NewLumbermillServer(&http.Server{}, nil)
	server.AddPrincipal("foo", "foo")

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
	server := NewLumbermillServer(&http.Server{}, NewHashRing(1, nil))
	server.AddPrincipal("foo", "foo")

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("")))
	req.SetBasicAuth("foo", "foo")
	if err != nil {
		t.Fatal(err)
	}

	server.http.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}
}

func TestTarget(t *testing.T) {
	hashRing, _, _ := createMessageRoutes("null", true)
	server := NewLumbermillServer(&http.Server{}, hashRing)
	server.AddPrincipal("foo", "foo")

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
