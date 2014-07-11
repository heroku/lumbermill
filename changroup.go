package main

import (
	"fmt"
	"time"

	metrics "github.com/rcrowley/go-metrics"
)

// A channel of points and related sampling
type ChanGroup struct {
	Name       string
	points     chan Point
	depthGauge metrics.Gauge
}

func NewChanGroup(name string, chanCap int) *ChanGroup {
	group := &ChanGroup{Name: name}
	group.points = make(chan Point, chanCap)
	group.depthGauge = metrics.NewRegisteredGauge(
		fmt.Sprintf("lumbermill.points.pending.", name),
		metrics.DefaultRegistry,
	)

	go group.Sample(10 * time.Second)

	return group
}

// Update depth guages every so often
func (g *ChanGroup) Sample(every time.Duration) {
	for {
		time.Sleep(every)
		g.depthGauge.Update(int64(len(g.points)))
	}
}
