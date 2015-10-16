package destination

import (
	"time"

	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/heroku/logma"
	metrics "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/rcrowley/go-metrics"
)

var (
	droppedErrorCounter = metrics.GetOrRegisterCounter("lumbermill.errors.dropped", metrics.DefaultRegistry)
)

// A channel of points and related sampling
type destination struct {
	Name       string
	envelopes  chan *logma.Envelope
	depthGauge metrics.Gauge
}

func newDestination(name string, chanCap int) *destination {
	destination := &destination{Name: name}
	destination.envelopes = make(chan *logma.Envelope, chanCap)
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
		d.depthGauge.Update(int64(len(d.envelopes)))
	}
}

// Post the point, or increment a counter if channel is full
func (d *destination) Post(envelope *logma.Envelope) {
	select {
	case d.envelopes <- envelope:
	default:
		droppedErrorCounter.Inc(1)
	}
}

func (d *destination) Close() error {
	close(d.envelopes)
	return nil
}
