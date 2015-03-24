package authenticater

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"sync"
	"testing"
)

// run with -race
func TestLogplexDrainTokenRace(t *testing.T) {
	ldta := NewLogplexDrainToken()
	wg := sync.WaitGroup{}
	for i := 0; i <= 10000; i++ {
		wg.Add(1)
		go func(v int) {
			ldta.AddToken("test" + strconv.Itoa(v))
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
				r.Header.Add("Logplex-Drain-Token", "test"+strconv.Itoa(v))
				if !ldta.Authenticate(r) {
					log.Fatalf("Unable to authenticate request #%d\n", v)
				}
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}
