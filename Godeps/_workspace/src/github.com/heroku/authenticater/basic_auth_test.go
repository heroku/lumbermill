package authenticater

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"sync"
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

// run with -race
func TestBasicAuthRace(t *testing.T) {
	ba := NewBasicAuth()
	wg := sync.WaitGroup{}
	for i := 0; i <= 10000; i++ {
		wg.Add(1)
		go func(v int) {
			t := strconv.Itoa(v)
			ba.AddPrincipal("test"+t, "pass"+t)
			wg.Done()
		}(i)
	}
	wg.Wait()
	for i := 0; i <= 10000; i++ {
		wg.Add(1)
		go func(v int) {
			if r, err := http.NewRequest("GET", "/", bytes.NewBufferString("")); err != nil {
				log.Fatalf("Unable to create request #%d\n", v)
			} else {
				t := strconv.Itoa(v)
				r.SetBasicAuth("test"+t, "pass"+t)
				if !ba.Authenticate(r) {
					log.Fatalf("Unable to authenticate request #%d\n", v)
				}
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}
