package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	metrics "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/rcrowley/go-metrics"
)

const (
	libratoBacklog         = 8 // No more than N pending batches in-flight
	libratoMaxAttempts     = 4 // Max attempts before dropping batch
	libratoStartingBackoff = 500 * time.Millisecond
	libratoMetricsUrl      = "https://metrics-api.librato.com/v1/metrics"
)

type libratoPoster struct {
	client               *http.Client
	destination          *destination
	libratoUser          string
	libratoToken         string
	pointsSuccessCounter metrics.Counter
	pointsSuccessTime    metrics.Timer
	pointsFailureCounter metrics.Counter
	pointsFailureTime    metrics.Timer
}

type libratoMetric struct {
	Name   string      `json:"name"`
	Value  interface{} `json:"value"`
	When   int64       `json:"measure_time"`
	Source string      `json:"source,omitempty"`
}

type libratoPayload struct {
	Gauges []libratoMetric `json:"gauges,omitempty"`
}

func newLibratoPoster(libratoUser, libratoToken string, client *http.Client, destination *destination, waitGroup *sync.WaitGroup) *libratoPoster {
	return &libratoPoster{
		client:               client,
		destination:          destination,
		libratoUser:          libratoUser,
		libratoToken:         libratoToken,
		pointsSuccessCounter: metrics.GetOrRegisterCounter("lumbermill.poster.deliver.points.lumbrato", metrics.DefaultRegistry),
		pointsSuccessTime:    metrics.GetOrRegisterTimer("lumbermill.poster.success.time.lumbrato", metrics.DefaultRegistry),
		pointsFailureCounter: metrics.GetOrRegisterCounter("lumbermill.poster.error.points.lumbrato", metrics.DefaultRegistry),
		pointsFailureTime:    metrics.GetOrRegisterTimer("lumbermill.poster.error.time.lumbrato", metrics.DefaultRegistry),
	}
}

func (p *libratoPoster) Run() {
	var last bool
	var delivery *libratoPayload

	timeout := time.NewTicker(time.Second)
	defer func() { timeout.Stop() }()

	for !last {
		delivery, last = p.nextDelivery(timeout)
		p.deliver(delivery)
	}
}

func pointsToPayload(p point, payload *libratoPayload) {
	name := "lumbrato." + p.Type.Name()
	source := "lumbrato." + md5sum([]byte(p.Token))

	tstamp := p.Points[0].(int64) / 1000

	switch p.Type {
	case routerRequest:
		payload.Gauges = append(payload.Gauges, libratoMetric{
			Name:   fmt.Sprintf("%s.%d", name, p.Points[1]), // status code
			Source: source,
			When:   tstamp,
			Value:  p.Points[2].(int), // service
		})
	case routerEvent:
		payload.Gauges = append(payload.Gauges, libratoMetric{
			Name:   name + "." + p.Points[1].(string),
			Source: source,
			When:   tstamp,
			Value:  1,
		})
	case dynoMem:
		sourceDyno := source + "." + p.Points[1].(string)
		for i := 2; i < 8; i++ {
			payload.Gauges = append(payload.Gauges, libratoMetric{
				Name:   name + "." + dynoMem.Columns()[i],
				Source: sourceDyno,
				When:   tstamp,
				Value:  p.Points[i].(float64),
			})
		}
	case dynoLoad:
		sourceDyno := source + "." + p.Points[1].(string)
		for i := 2; i < 6; i++ {
			payload.Gauges = append(payload.Gauges, libratoMetric{
				Name:   name + "." + dynoLoad.Columns()[i],
				Source: sourceDyno,
				When:   tstamp,
				Value:  p.Points[i].(float64),
			})
		}

	case dynoEvents:
		payload.Gauges = append(payload.Gauges, libratoMetric{
			Name:   fmt.Sprintf("%s.%s.%d", name, p.Points[2], p.Points[3]),
			Source: source,
			When:   tstamp,
			Value:  1,
		})
	}
}

func (p *libratoPoster) nextDelivery(timeout *time.Ticker) (delivery *libratoPayload, last bool) {
	delivery = new(libratoPayload)
	for {
		select {
		case point, open := <-p.destination.points:
			if open {
				pointsToPayload(point, delivery)
			} else {
				return delivery, true
			}
		case <-timeout.C:
			return delivery, false
		}
	}
}

func (p *libratoPoster) deliver(payload *libratoPayload) {
	pointCount := len(payload.Gauges)

	if pointCount == 0 {
		return
	}

	start := time.Now()

	j, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Error while marshaling payload to json. at=libratoPoster.deliver err=%q", err)
	}

	if !p.sendWithBackoff(j) {
		p.pointsFailureCounter.Inc(1)
		p.pointsFailureTime.UpdateSince(start)
		log.Printf("Error posting points: %s\n", err)
	} else {
		p.pointsSuccessCounter.Inc(1)
		p.pointsSuccessTime.UpdateSince(start)
		deliverySizeHistogram.Update(int64(pointCount))
	}
}

func (p *libratoPoster) sendWithBackoff(payload []byte) bool {
	attempts := 0
	cBackoff := libratoStartingBackoff

	for attempts < libratoMaxAttempts {
		retry, err := p.send(payload)
		if retry {
			fmt.Printf("fn=sendWithBackoff poster=librato backoff=%d attempts=%d err=%q", cBackoff, attempts, err)
			cBackoff = backoff(cBackoff)
		} else {
			if err != nil {
				fmt.Printf("fn=sendWithBackoff poster=librato backoff=%d attempts=%d err=%q", cBackoff, attempts, err)
				return false
			} else {
				return true
			}
		}
		attempts += 1
	}
	return false
}

// Attempts to send the payload and signals retries on errors
func (p *libratoPoster) send(payload []byte) (bool, error) {
	body := bytes.NewReader(payload)
	req, err := http.NewRequest("POST", libratoMetricsUrl, body)
	if err != nil {
		return false, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "lumbermill/librato-output")
	req.SetBasicAuth(p.libratoUser, p.libratoToken)

	// TODO: Maybe we want our own client?
	resp, err := p.client.Do(req)
	if err != nil {
		return true, err
	} else {
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			b, _ := ioutil.ReadAll(resp.Body)

			if resp.StatusCode >= 500 {
				err = fmt.Errorf("server error: %d, body: %+q", resp.StatusCode, string(b))
				return true, err
			} else {
				err = fmt.Errorf("client error: %d, body: %+q", resp.StatusCode, string(b))
				return false, err
			}
		}
	}

	return false, nil
}

// Sleeps `bo` and then returns double
func backoff(bo time.Duration) time.Duration {
	time.Sleep(bo)
	return bo * 2
}
