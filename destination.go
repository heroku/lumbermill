package main

import (
	"time"

	metrics "github.com/rcrowley/go-metrics"
)

// A channel of points and related sampling
type Destination struct {
	Name       string
	points     chan Point
	depthGauge metrics.Gauge
}

func NewDestination(name string, chanCap int) *Destination {
	destination := &Destination{Name: name}
	destination.points = make(chan Point, chanCap)
	destination.depthGauge = metrics.NewRegisteredGauge(
		"lumbermill.points.pending."+name,
		metrics.DefaultRegistry,
	)

	go destination.Sample(10 * time.Second)

	return destination
}

// Update depth guages every so often
func (d *Destination) Sample(every time.Duration) {
	for {
		time.Sleep(every)
		d.depthGauge.Update(int64(len(d.points)))
	}
}

// Post the point, or increment a counter if channel is full
func (d *Destination) PostPoint(point Point) {
	select {
	case d.points <- point:
	default:
		droppedErrorCounter.Inc(1)
	}
}

func (d *Destination) Close() error {
	close(d.points)
	return nil
}