package main

import (
	"time"

	metrics "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/rcrowley/go-metrics"
)

// A channel of points and related sampling
type destination struct {
	Name       string
	points     chan point
	depthGauge metrics.Gauge
}

func newDestination(name string, chanCap int) *destination {
	destination := &destination{Name: name}
	destination.points = make(chan point, chanCap)
	destination.depthGauge = metrics.GetOrRegisterGauge(
		"lumbermill.points.pending."+name,
		metrics.DefaultRegistry,
	)

	go destination.Sample(10 * time.Second)

	return destination
}

// Update depth guages every so often
func (d *destination) Sample(every time.Duration) {
	for {
		time.Sleep(every)
		d.depthGauge.Update(int64(len(d.points)))
	}
}

// Post the point, or increment a counter if channel is full
func (d *destination) PostPoint(point point) {
	select {
	case d.points <- point:
	default:
		droppedErrorCounter.Inc(1)
	}
}

func (d *destination) Close() error {
	close(d.points)
	return nil
}
