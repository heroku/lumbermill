package main

import (
	"fmt"
	"time"

	metrics "github.com/rcrowley/go-metrics"
)

type ChanGroup struct {
	Name        string
	points      []chan []interface{}
	depthGauges []metrics.Gauge
}

func NewChanGroup(name string, chanCap int) *ChanGroup {
	group := &ChanGroup{Name: name}
	group.points = make([]chan []interface{}, numSeries)
	group.depthGauges = make([]metrics.Gauge, numSeries)

	for i := 0; i < numSeries; i++ {
		group.points[i] = make(chan []interface{}, chanCap)
		group.depthGauges[i] = metrics.NewGauge()
		metrics.DefaultRegistry.Register(
			fmt.Sprintf("lumbermill.points.%s.pending", seriesNames[i]),
			group.depthGauges[i],
		)
	}

	go group.Sample(10 * time.Second)

	return group
}

// Update depth guages every so often
func (g *ChanGroup) Sample(every time.Duration) {
	for {
		time.Sleep(every)
		for i, gauge := range g.depthGauges {
			gauge.Update(int64(len(g.points[i])))
		}
	}
}
