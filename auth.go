package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

type Auther interface {
	AddPrincipal(user, pass string)
	Authenticate(user, pass string) bool
}

func extractBasicAuth(r *http.Request) (user, pass string, status int) {
	status = http.StatusOK
	header := r.Header.Get("Authorization")
	if header == "" {
		status = http.StatusForbidden
		return
	}
	headerParts := strings.SplitN(header, " ", 2)
	if len(headerParts) != 2 {
		status = http.StatusBadRequest
		return
	}

	method := headerParts[0]
	if method != "Basic" {
		status = http.StatusBadRequest
		return
	}

	encodedUserPass := headerParts[1]
	decodedUserPass, err := base64.StdEncoding.DecodeString(encodedUserPass)
	if err != nil {
		status = http.StatusBadRequest
		return
	}

	userPassParts := strings.SplitN(string(decodedUserPass), ":", 2)
	if len(userPassParts) != 2 {
		status = http.StatusBadRequest
		return
	}

	user = userPassParts[0]
	pass = userPassParts[1]

	return
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
		user, pass, status := extractBasicAuth(r)
		if status != http.StatusOK {
			w.WriteHeader(status)
		}

		if !auth.Authenticate(user, pass) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		handle(w, r)
	}
}
