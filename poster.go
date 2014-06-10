package main

import (
	"log"
	"time"

	"github.com/heroku/slog"
	influx "github.com/influxdb/influxdb-go"
)

type Poster struct {
	points       <-chan []interface{}
	name         string
	columns      []string
	influxClient *influx.Client
}

func NewPoster(clientConfig influx.ClientConfig, name string, points <-chan []interface{}, columns []string) *Poster {
	influxClient, err := influx.NewClient(&influxClientConfig)

	if err != nil {
		panic(err)
	}

	return &Poster{
		points:       points,
		name:         name,
		columns:      columns,
		influxClient: influxClient,
	}
}

func (p *Poster) Run() {
	timeout := time.NewTicker(time.Second)
	defer func() { timeout.Stop() }()

	series := &influx.Series{Points: make([][]interface{}, 0)}
	series.Name = p.name
	series.Columns = p.columns

	for {
		select {
		case point, open := <-p.points:
			if open {
				series.Points = append(series.Points, point)
			} else {
				break
			}
		case <-timeout.C:
			if len(series.Points) > 0 {
				p.deliver(series)
			}
		}
	}

	if len(series.Points) > 0 {
		p.deliver(series)
	}
}

func (p *Poster) deliver(series *influx.Series) {
	ctx := slog.Context{}
	defer func() { LogWithContext(ctx) }()

	postableSeries := make([]*influx.Series, 0, 1)
	postableSeries = append(postableSeries, series)

	start := time.Now()
	err := p.influxClient.WriteSeriesWithTimePrecision(postableSeries, influx.Microsecond)
	if err != nil {
		ctx.Count("poster.error."+p.name+".points", len(series.Points))
		ctx.MeasureSince("poster.error."+p.name+".time", start)
		log.Println(err)
	} else {
		ctx.Count("poster.deliver."+p.name+".points", len(series.Points))
		ctx.MeasureSince("poster.success."+p.name+".time", start)
	}

	series.Points = series.Points[0:0]
}
