package main

import (
	"log"
	"time"

	influx "github.com/influxdb/influxdb-go"
	metrics "github.com/rcrowley/go-metrics"
)

type Poster struct {
	chanGroup            *ChanGroup
	name                 string
	influxClient         *influx.Client
	pointsSuccessCounter metrics.Counter
	pointsSuccessTime    metrics.Timer
	pointsFailureCounter metrics.Counter
	pointsFailureTime    metrics.Timer
}

func NewPoster(clientConfig influx.ClientConfig, name string, chanGroup *ChanGroup) *Poster {
	influxClient, err := influx.NewClient(&clientConfig)

	if err != nil {
		panic(err)
	}

	return &Poster{
		chanGroup:            chanGroup,
		name:                 name,
		influxClient:         influxClient,
		pointsSuccessCounter: metrics.NewRegisteredCounter("lumbermill.poster.deliver.points."+name, metrics.DefaultRegistry),
		pointsSuccessTime:    metrics.NewRegisteredTimer("lumbermill.poster.success.time."+name, metrics.DefaultRegistry),
		pointsFailureCounter: metrics.NewRegisteredCounter("lumbermill.poster.error.points."+name, metrics.DefaultRegistry),
		pointsFailureTime:    metrics.NewRegisteredTimer("lumbermill.poster.error.time."+name, metrics.DefaultRegistry),
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
			p.deliver(seriesGroup)
		}
	}

	p.deliver(seriesGroup)
}

func (p *Poster) deliver(seriesGroup []*influx.Series) {
	pointCount := 0
	for _, s := range seriesGroup {
		pointCount += len(s.Points)
	}

	if pointCount == 0 {
		return
	}

	start := time.Now()
	err := p.influxClient.WriteSeriesWithTimePrecision(seriesGroup, influx.Microsecond)

	if err != nil {
		p.pointsFailureCounter.Inc(1)
		p.pointsFailureTime.UpdateSince(start)
		log.Println(err)
	} else {
		p.pointsSuccessCounter.Inc(1)
		p.pointsSuccessTime.UpdateSince(start)
	}

	for _, series := range seriesGroup {
		series.Points = series.Points[0:0]
	}
}
