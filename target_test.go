package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTargetWithoutAuth(t *testing.T) {
	server := NewLumbermillServer(&http.Server{}, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("")))
	if err != nil {
		t.Fatal(err)
	}

	server.serveTarget(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}
}

func TestTargetWithoutId(t *testing.T) {
	//Setup
	User = "foo"
	Password = "foo"
	server := NewLumbermillServer(&http.Server{}, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/", bytes.NewReader([]byte("")))
	req.SetBasicAuth("foo", "foo")
	if err != nil {
		t.Fatal(err)
	}

	server.serveTarget(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}
}

func TestTargetWithoutRing(t *testing.T) {
	//Setup
	User = "foo"
	Password = "foo"

	server := NewLumbermillServer(&http.Server{}, NewHashRing(1, nil))

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("")))
	req.SetBasicAuth("foo", "foo")
	if err != nil {
		t.Fatal(err)
	}

	server.serveTarget(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}
}

func TestTarget(t *testing.T) {
	//Setup
	User = "foo"
	Password = "foo"

	hashRing, _ := createMessageRoutes("null")
	server := NewLumbermillServer(&http.Server{}, hashRing)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/target/foo", bytes.NewReader([]byte("")))
	req.SetBasicAuth("foo", "foo")
	if err != nil {
		t.Fatal(err)
	}

	server.serveTarget(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatal("Wrong Response Code: ", recorder.Code)
	}

	body := recorder.Body.String()

	if body != "{ \"host\": \"null\" }" {
		t.Fatal("Wrong Body: ", body)
	}
}
