package main

import (
	"fmt"
	"net/http"
	"strings"
)

// GET /target/<opaque id>
func (s *server) serveTarget(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.URL.Path, "/", 3)
	if len(parts) != 3 || parts[2] == "" {
		w.WriteHeader(http.StatusBadRequest)
		badRequestCounter.Inc(1)
		return
	}

	id := parts[2]

	destination := s.hashRing.Get(id)

	if destination == nil {
		w.WriteHeader(http.StatusInternalServerError)
		internalServerErrorCounter.Inc(1)
		return
	}

	response := []byte("{ \"host\": \"" + destination.Name + "\" }")
	headers := w.Header()
	headers.Set("Content-Length", fmt.Sprintf("%d", len(response)))
	headers.Set("Content-Type", "application/json")
	w.Write(response)
}
