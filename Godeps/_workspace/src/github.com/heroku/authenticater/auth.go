package authenticater

import "net/http"

// Authenticater provides an interface for authentication of a http.Request
type Authenticater interface {
	Authenticate(r *http.Request) bool
}

// WrapAuth returns a http.Handlerfunc that runs the passed Handlerfunc if and
// only if the Authenticator can authenticate the request
func WrapAuth(auth Authenticater, handle http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if auth.Authenticate(r) {
			handle(w, r)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}
}
