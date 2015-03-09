package main

import (
	"fmt"
	"net/http"
	"strings"
)

// Auther provides an interface for authentication
type Auther interface {
	AddPrincipal(user, pass string)
	Authenticate(user, pass string) bool
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

func wrapBasicAuth(auth Auther, handle http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			authFailureCounter.Inc(1)
			return
		}

		if !auth.Authenticate(user, pass) {
			w.WriteHeader(http.StatusUnauthorized)
			authFailureCounter.Inc(1)
			return
		}

		handle(w, r)
	}
}
