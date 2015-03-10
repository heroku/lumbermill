package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

type LumbermillServer struct {
	sync.WaitGroup
	connectionCloser chan struct{}
	hashRing         *HashRing
	http             *http.Server
	shutdownChan     ShutdownChan
	isShuttingDown   bool
	credStore        map[string]string
}

func NewLumbermillServer(server *http.Server, auth Authenticater, hashRing *HashRing) *LumbermillServer {
	s := &LumbermillServer{
		connectionCloser: make(chan struct{}),
		shutdownChan:     make(chan struct{}),
		http:             server,
		hashRing:         hashRing,
		credStore:        make(map[string]string),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/drain", wrapAuth(auth,
		func(w http.ResponseWriter, r *http.Request) {
			s.serveDrain(w, r)
			s.recycleConnection(w)
		}))

	mux.HandleFunc("/health", s.serveHealth)
	mux.HandleFunc("/target/", wrapAuth(auth, s.serveTarget))

	s.http.Handler = mux

	return s
}

func (s *LumbermillServer) Close() error {
	s.shutdownChan <- struct{}{}
	return nil
}

func (s *LumbermillServer) scheduleConnectionRecycling(after time.Duration) {
	for !s.isShuttingDown {
		time.Sleep(after)
		s.connectionCloser <- struct{}{}
	}
}

func (s *LumbermillServer) recycleConnection(w http.ResponseWriter) {
	select {
	case <-s.connectionCloser:
		w.Header().Set("Connection", "close")
	default:
		if s.isShuttingDown {
			w.Header().Set("Connection", "close")
		}
	}
}

func (s *LumbermillServer) Run(connRecycle time.Duration) {
	go s.awaitShutdown()
	go s.scheduleConnectionRecycling(connRecycle)

	if err := s.http.ListenAndServe(); err != nil {
		log.Fatalln("Unable to start HTTP server: ", err)
	}
}

// Health Checks, so just say 200 - OK
// TODO: Actual healthcheck
func (s *LumbermillServer) serveHealth(w http.ResponseWriter, r *http.Request) {
	if s.isShuttingDown {
		http.Error(w, "Shutting Down", 503)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *LumbermillServer) awaitShutdown() {
	<-s.shutdownChan
	log.Printf("Shutting down.")
	s.isShuttingDown = true
}
