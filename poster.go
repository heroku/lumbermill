package main

import (
	"log"
	"time"

	"github.com/heroku/slog"
	influx "github.com/influxdb/influxdb-go"
)

type Poster struct {
	chanGroup    *ChanGroup
	name         string
	influxClient *influx.Client
}

func NewPoster(clientConfig influx.ClientConfig, name string, chanGroup *ChanGroup) *Poster {
	influxClient, err := influx.NewClient(&influxClientConfig)

	if err != nil {
		panic(err)
	}

	return &Poster{
		chanGroup:    chanGroup,
		name:         name,
		influxClient: influxClient,
	}
}

func (p *Poster) Run() {
	timeout := time.NewTicker(time.Second)
	defer func() { timeout.Stop() }()

	seriesGroup := make([]*influx.Series, numSeries)

	for i := 0; i < numSeries; i++ {
		series := &influx.Series{Points: make([][]interface{}, 0)}
		series.Name = seriesNames[i]
		series.Columns = seriesColumns[i]
		seriesGroup[i] = series
	}

	for {
		select {
		case point, open := <-p.chanGroup.points[Router]:
			if open {
				seriesGroup[Router].Points = append(seriesGroup[Router].Points, point)
			} else {
				break
			}
		case point, open := <-p.chanGroup.points[EventsRouter]:
			if open {
				seriesGroup[EventsRouter].Points = append(seriesGroup[EventsRouter].Points, point)
			} else {
				break
			}
		case point, open := <-p.chanGroup.points[DynoMem]:
			if open {
				seriesGroup[DynoMem].Points = append(seriesGroup[DynoMem].Points, point)
			} else {
				break
			}
		case point, open := <-p.chanGroup.points[DynoLoad]:
			if open {
				seriesGroup[DynoLoad].Points = append(seriesGroup[DynoLoad].Points, point)
			} else {
				break
			}
		case point, open := <-p.chanGroup.points[EventsDyno]:
			if open {
				seriesGroup[EventsDyno].Points = append(seriesGroup[EventsDyno].Points, point)
			} else {
				break
			}
		case <-timeout.C:
			pointCount := 0
			for _, s := range seriesGroup {
				pointCount += len(s.Points)
			}

			if pointCount > 0 {
				p.deliver(seriesGroup, pointCount)
			}
		}
	}

	pointCount := 0
	for _, s := range seriesGroup {
		pointCount += len(s.Points)
	}

	if pointCount > 0 {
		p.deliver(seriesGroup, pointCount)
	}

}

func (p *Poster) deliver(seriesGroup []*influx.Series, count int) {
	ctx := slog.Context{}
	defer func() { LogWithContext(ctx) }()

	start := time.Now()
	err := p.influxClient.WriteSeriesWithTimePrecision(seriesGroup, influx.Microsecond)

	// TODO(apg): Figure out how we want these metrics
	if err != nil {
		ctx.Count("poster.error."+p.name+".points", count)
		ctx.MeasureSince("poster.error."+p.name+".time", start)
		log.Println(err)
	} else {
		ctx.Count("poster.deliver."+p.name+".points", count)
		ctx.MeasureSince("poster.success."+p.name+".time", start)
	}

	for _, series := range seriesGroup {
		series.Points = series.Points[0:0]
	}
}
