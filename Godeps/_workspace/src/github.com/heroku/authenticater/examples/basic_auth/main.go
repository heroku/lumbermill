package main

import (
	"net/http"

	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/heroku/authenticater"
)

func main() {
	auth := authenticater.NewBasicAuth()
	auth.AddPrincipal("foo", "bar")
	http.HandleFunc("/", authenticater.WrapAuth(auth,
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello"))
		}))
	http.ListenAndServe(":8080", nil)
}

// curl -v http://localhost:8080/foo
// 401
//
// curl -v http://foo:bar@localhost:8080/foo
// 200
