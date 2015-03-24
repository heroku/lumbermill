package authenticater

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// BasicAuth handles normal user/password Basic Auth requests, multiple
// password for the same user and is safe for concurrent use.
type BasicAuth struct {
	sync.RWMutex
	creds map[string][]string
}

// NewBasicAuth returns an empty BasicAuth Authenticator
func NewBasicAuth() *BasicAuth {
	return &BasicAuth{
		creds: make(map[string][]string),
	}
}

// NewBasicAuthFromString creates and populates a BasicAuth from the provided
// credentials, encoded as a string, in the following format:
// user:password|user:password|...
func NewBasicAuthFromString(creds string) (*BasicAuth, error) {
	ba := NewBasicAuth()
	for _, u := range strings.Split(creds, "|") {
		uparts := strings.SplitN(u, ":", 2)
		if len(uparts) != 2 || len(uparts[0]) == 0 || len(uparts[1]) == 0 {
			return ba, fmt.Errorf("Unable to create credentials from '%s'", u)
		}

		ba.AddPrincipal(uparts[0], uparts[1])
	}
	return ba, nil
}

// AddPrincipal add's a user/password combo to the list of valid combinations
func (ba *BasicAuth) AddPrincipal(user, pass string) {
	ba.Lock()
	u, existed := ba.creds[user]
	if !existed {
		u = make([]string, 0, 1)
	}
	ba.creds[user] = append(u, pass)
	ba.Unlock()
}

// Authenticate is true if the Request has a valid BasicAuth signature and
// that signature encodes a known username/password combo.
func (ba *BasicAuth) Authenticate(r *http.Request) bool {
	user, pass, ok := r.BasicAuth()
	if !ok {
		return false
	}

	ba.RLock()
	defer ba.RUnlock()

	if passwords, exists := ba.creds[user]; exists {
		for _, password := range passwords {
			if password == pass {
				return true
			}
		}
	}

	return false
}
