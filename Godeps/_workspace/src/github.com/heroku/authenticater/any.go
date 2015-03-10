package authenticater

import "net/http"

type AnyOrNoAuth struct{}

func (fa AnyOrNoAuth) Authenticate(r *http.Request) bool {
	return true
}
