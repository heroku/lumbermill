package main

import (
	"fmt"
	"github.com/heroku/slog"
)

type ChanGroup struct {
	Name string
	points []chan []interface{}
}

func NewChanGroup(name string, chanCap int) *ChanGroup {
	group := &ChanGroup{Name: name}
	group.points = make([]chan []interface{}, numSeries)

	for i := 0; i < numSeries; i++ {
		group.points[i] = make(chan []interface{}, chanCap)
	}

	return group
}

func (g *ChanGroup) Sample(ctx slog.Context) {
	for i := 0; i < numSeries; i++ {
		// TODO: If we set the ChanGroup.Name to be the hostname, this might need to change.
		ctx.Sample(fmt.Sprintf("points.%s.%s.pending", g.Name, seriesNames[i]), len(g.points[i]))
	}
}
