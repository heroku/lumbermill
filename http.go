package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type HttpServer struct {
	sync.WaitGroup
	ConnectionCloser chan struct{}
	shutdownChan   ShutdownChan
	isShuttingDown bool
}

func NewHttpServer() *HttpServer {
	return &HttpServer{
	  ConnectionCloser: make(chan struct{}),
	  shutdownChan: make(chan struct{}),
	}
}

func (s *HttpServer) Signal() {
	s.shutdownChan <- struct{}{}
}

func (s *HttpServer) RecycleConnections(after time.Duration) {
	for !s.isShuttingDown {
		time.Sleep(after)
		s.ConnectionCloser <- struct{}{}
	}
}

func (s *HttpServer) Run(port string, connRecycle time.Duration) {
	go s.awaitShutdown()

	http.HandleFunc("/drain", s.serveDrain)
	http.HandleFunc("/health", s.serveHealth)
	http.HandleFunc("/target/", s.serveTarget)

	go s.RecycleConnections(connRecycle)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalln("Unable to start HTTP server: ", err)
	}
}

// Health Checks, so just say 200 - OK
// TODO: Actual healthcheck
func (s *HttpServer) serveHealth(w http.ResponseWriter, r *http.Request) {
	if s.isShuttingDown {
		http.Error(w, "Shutting Down", 503)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *HttpServer) awaitShutdown() {
	<- s.shutdownChan
	log.Printf("Shutting down.")
	s.isShuttingDown = true
}

func (s *HttpServer) checkAuth(r *http.Request) error {
	header := r.Header.Get("Authorization")
	if header == "" {
		return errors.New("Authorization required")
	}
	headerParts := strings.SplitN(header, " ", 2)
	if len(headerParts) != 2 {
		return errors.New("Authorization header is malformed")
	}

	method := headerParts[0]
	if method != "Basic" {
		return errors.New("Only Basic Authorization is accepted")
	}

	encodedUserPass := headerParts[1]
	decodedUserPass, err := base64.StdEncoding.DecodeString(encodedUserPass)
	if err != nil {
		return errors.New("Authorization header is malformed")
	}

	userPassParts := bytes.SplitN(decodedUserPass, []byte{':'}, 2)
	if len(userPassParts) != 2 {
		return errors.New("Authorization header is malformed")
	}

	user := userPassParts[0]
	pass := userPassParts[1]

	if string(user) != User {
		return errors.New("Unknown user")
	}
	if string(pass) != Password {
		return errors.New("Incorrect token")
	}

	return nil
}
