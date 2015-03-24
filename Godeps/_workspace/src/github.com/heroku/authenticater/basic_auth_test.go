package authenticater

import (
	"bytes"
	"log"
	"net/http"
	"testing"
)

var (
	// CREDS, user, passwords...
	positiveTests = [][]string{
		[]string{"foo:bar", "foo", "bar"},
		[]string{"foo:bar|bar:foo", "bar", "foo"},
		[]string{"foo:bar|foo:alterbar", "foo", "bar"},
		[]string{"foo:alterbar|foo:bar", "foo", "bar"},
	}

	negativeTests = [][]string{
		// CREDS, user, passwords...
		[]string{"foo:bar", "bar", "foo"},
		[]string{"foo:bar", "foo", ""},
		[]string{"foo:bar", "foo", "fizzle"},
		[]string{"foo:bar|bar:foo", "foozle", "bar"},
		[]string{"foo:alterbar|foo:bar", "bar", "car"},
	}
)

func TestBasicAuthPositives(t *testing.T) {
	for _, testValues := range positiveTests {
		ba, err := NewBasicAuthFromString(testValues[0])
		if err != nil {
			log.Fatalf("Unable to construct basic auth checker from creds (%s): %s\n", testValues[0], err)
		}

		r, err := http.NewRequest("GET", "/foo", bytes.NewBufferString(""))
		if err != nil {
			log.Fatalf("Unable to construct sample request: %s\n", err)
		}
		r.SetBasicAuth(testValues[1], testValues[2])

		if !ba.Authenticate(r) {
			log.Fatalf("Expected basic auth to work (CREDS = '%s', USER = '%s', PWD = '%s'), but didn't.", testValues[0], testValues[1], testValues[2])
		}
	}
}

func TestBasicAuthNegatives(t *testing.T) {
	for _, testValues := range negativeTests {
		ba, err := NewBasicAuthFromString(testValues[0])
		if err != nil {
			log.Fatalf("Unable to construct basic auth checker from creds (%s): %s\n", testValues[0], err)
		}

		r, err := http.NewRequest("GET", "/foo", bytes.NewBufferString(""))
		if err != nil {
			log.Fatalf("Unable to construct sample request: %s\n", err)
		}
		r.SetBasicAuth(testValues[1], testValues[2])

		if ba.Authenticate(r) {
			log.Fatalf("Expected basic auth to fail (CREDS = '%s', USER = '%s', PWD = '%s'), but didn't.", testValues[0], testValues[1], testValues[2])
		}
	}
}
