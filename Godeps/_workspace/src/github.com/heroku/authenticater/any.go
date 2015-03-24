package authenticater

import "net/http"

// AnyOrNoAuth just returns true for any call to Authenticate
type AnyOrNoAuth struct{}

// Authenticate all requests
func (fa AnyOrNoAuth) Authenticate(r *http.Request) bool {
	return true
}
