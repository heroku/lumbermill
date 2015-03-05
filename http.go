package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
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

func NewLumbermillServer(server *http.Server, hashRing *HashRing, creds string) *LumbermillServer {
	store, err := parseCreds(creds)
	if err != nil {
		log.Fatalln("Unable to create credentials")
	}

	s := &LumbermillServer{
		connectionCloser: make(chan struct{}),
		shutdownChan:     make(chan struct{}),
		http:             server,
		hashRing:         hashRing,
		credStore:        store,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/drain", func(w http.ResponseWriter, r *http.Request) {
		s.serveDrain(w, r)
		s.recycleConnection(w)
	})

	mux.HandleFunc("/health", s.serveHealth)
	mux.HandleFunc("/target/", s.serveTarget)

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

func (s *LumbermillServer) checkAuth(r *http.Request) error {
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

	if val, ok := s.credStore[string(user)]; !ok {
		return errors.New("Unable to authenticate")
	} else if val != string(pass) {
		return errors.New("Unable to authenticate")
	}

	return nil
}

// Parse creds expects a string user1:password1|user2:password2
func parseCreds(creds string) (map[string]string, error) {
	store := make(map[string]string)
	for _, u := range strings.Split(creds, "|") {
		uparts := strings.SplitN(u, ":", 2)
		if len(uparts) != 2 || len(uparts[0]) == 0 || len(uparts[1]) == 0 {
			return store, fmt.Errorf("Unable to create credentials from '%s'", u)
		}

		store[uparts[0]] = uparts[1]
	}
	return store, nil
}
