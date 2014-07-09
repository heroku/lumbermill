package main

import (
	"fmt"
	"net/http"
	"strings"
)

// GET /target/<opaque id>
func serveTarget(w http.ResponseWriter, r *http.Request) {
	if err := checkAuth(r); err != nil {
		w.WriteHeader(http.StatusForbidden)
		authFailureCounter.Inc(1)
		return
	}

	parts := strings.SplitN(r.URL.Path, "/", 3)
	if len(parts) != 3 || parts[2] == "" {
		w.WriteHeader(http.StatusBadRequest)
		badRequestCounter.Inc(1)
		return
	}

	id := parts[2]

	chanGroup := hashRing.Get(id)

	if chanGroup == nil {
		w.WriteHeader(http.StatusInternalServerError)
		internalServerErrorCounter.Inc(1)
		return
	}

	response := []byte("{ \"host\": \"" + chanGroup.Name + "\" }")
	headers := w.Header()
	headers.Set("Content-Length", fmt.Sprintf("%d", len(response)))
	headers.Set("Content-Type", "application/json")
	w.Write(response)
}
