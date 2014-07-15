package main

import (
	"log"
	"time"

	influx "github.com/influxdb/influxdb-go"
	metrics "github.com/rcrowley/go-metrics"
)

var deliverySizeHistogram = metrics.NewRegisteredHistogram("lumbermill.poster.deliver.sizes", metrics.DefaultRegistry, metrics.NewUniformSample(100))

type Poster struct {
	destination          *Destination
	name                 string
	influxClient         *influx.Client
	pointsSuccessCounter metrics.Counter
	pointsSuccessTime    metrics.Timer
	pointsFailureCounter metrics.Counter
	pointsFailureTime    metrics.Timer
}

func NewPoster(clientConfig influx.ClientConfig, name string, destination *Destination) *Poster {
	influxClient, err := influx.NewClient(&clientConfig)

	if err != nil {
		panic(err)
	}

	return &Poster{
		destination:          destination,
		name:                 name,
		influxClient:         influxClient,
		pointsSuccessCounter: metrics.NewRegisteredCounter("lumbermill.poster.deliver.points."+name, metrics.DefaultRegistry),
		pointsSuccessTime:    metrics.NewRegisteredTimer("lumbermill.poster.success.time."+name, metrics.DefaultRegistry),
		pointsFailureCounter: metrics.NewRegisteredCounter("lumbermill.poster.error.points."+name, metrics.DefaultRegistry),
		pointsFailureTime:    metrics.NewRegisteredTimer("lumbermill.poster.error.time."+name, metrics.DefaultRegistry),
	}
}

func makeSeries(p Point) *influx.Series {
	series := &influx.Series{Points: make([][]interface{}, 0)}
	series.Name = p.SeriesName()
	series.Columns = seriesColumns[p.Type]
	return series
}

func (p *Poster) Run() {
	timeout := time.NewTicker(time.Second)
	defer func() { timeout.Stop() }()

	allSeries := make(map[string]*influx.Series)

	for {
		select {
		case point, open := <-p.destination.points:
			if open {
				seriesName := point.SeriesName()
				series, found := allSeries[seriesName]
				if !found {
					series = makeSeries(point)
				}
				series.Points = append(series.Points, point.Points)
				allSeries[seriesName] = series
			} else {
				break
			}
		case <-timeout.C:
			p.deliver(allSeries)
			allSeries = make(map[string]*influx.Series)
		}
	}

	p.deliver(allSeries)
}

func (p *Poster) deliver(allSeries map[string]*influx.Series) {
	pointCount := 0
	seriesGroup := make([]*influx.Series, 0, len(allSeries))

	for _, s := range allSeries {
		pointCount += len(s.Points)
		seriesGroup = append(seriesGroup, s)
	}

	if pointCount == 0 {
		return
	}

	start := time.Now()
	err := p.influxClient.WriteSeriesWithTimePrecision(seriesGroup, influx.Microsecond)

	if err != nil {
		// TODO: Ugh. These could be timeout errors, or an internal error.
		//       Should probably attempt to figure out which...
		p.pointsFailureCounter.Inc(1)
		p.pointsFailureTime.UpdateSince(start)
		log.Println(err)
	} else {
		p.pointsSuccessCounter.Inc(1)
		p.pointsSuccessTime.UpdateSince(start)
		deliverySizeHistogram.Update(int64(pointCount))
	}
}
