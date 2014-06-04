package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/bmizerany/lpx"
	"github.com/heroku/slog"
	influx "github.com/influxdb/influxdb-go"
	"github.com/kr/logfmt"
)

var (
	influxClientConfig influx.ClientConfig
	influxClient       *influx.Client
)

func init() {
	var err error

	influxClientConfig = influx.ClientConfig{
		Host:     os.Getenv("INFLUXDB_HOST"), //"influxor.ssl.edward.herokudev.com:8086",
		Username: os.Getenv("INFLUXDB_USER"), //"test",
		Password: os.Getenv("INFLUXDB_PWD"),  //"tester",
		Database: os.Getenv("INFLUXDB_NAME"), //"ingress",
		IsSecure: true,
		HttpClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: os.Getenv("INFLUXDB_SKIP_VERIFY") == "true"},
				ResponseHeaderTimeout: 5 * time.Second,
				Dial: func(network, address string) (net.Conn, error) {
					return net.DialTimeout(network, address, 5*time.Second)
				},
			},
		},
	}

	influxClient, err = influx.NewClient(&influxClientConfig)
	if err != nil {
		fmt.Println(err)
	}
}

// "Parse tree" from hell
func serveDrain(w http.ResponseWriter, r *http.Request) {
	ctx := slog.Context{}
	defer func() { LogWithContext(ctx) }()

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx.Count("batch", 1)

	series := make([]*influx.Series, 0, 10)
	routerSeries := &influx.Series{Points: make([][]interface{}, 0)}
	routerEventSeries := &influx.Series{Points: make([][]interface{}, 0)}
	dynoMemSeries := &influx.Series{Points: make([][]interface{}, 0)}
	dynoLoadSeries := &influx.Series{Points: make([][]interface{}, 0)}
	dynoEvents := &influx.Series{Points: make([][]interface{}, 0)}

	routerSeriesWithId := &influx.Series{Points: make([][]interface{}, 0)}
	routerEventSeriesWithId := &influx.Series{Points: make([][]interface{}, 0)}
	dynoMemSeriesWithId := &influx.Series{Points: make([][]interface{}, 0)}
	dynoLoadSeriesWithId := &influx.Series{Points: make([][]interface{}, 0)}
	dynoEventsWithId := &influx.Series{Points: make([][]interface{}, 0)}

	//FIXME: Better auth? Encode the Token via Fernet and make that the user or password?
	id := r.Header.Get("Logplex-Drain-Token")

	parseStart := time.Now()
	lp := lpx.NewReader(bufio.NewReader(r.Body))
	defer r.Body.Close()

	for lp.Next() {
		ctx.Count("total.lines", 1)
		header := lp.Header()
		msg := lp.Bytes()
		switch string(header.Name) {
		case "heroku":
			t, e := time.Parse("2006-01-02T15:04:05.000000+00:00", string(lp.Header().Time))
			if e != nil {
				log.Printf("Error Parsing Time(%s): %q\n", string(lp.Header().Time), e)
				continue
			}
			timestamp := t.UnixNano() / int64(time.Microsecond)

			pid := string(header.Procid)
			switch pid {
			case "router":

				switch {
				// router logs with a H error code in them
				case bytes.Contains(msg, keyCodeH):
					ctx.Count("router.error.lines", 1)
					re := routerError{}
					err := logfmt.Unmarshal(msg, &re)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					routerEventSeries.Points = append(
						routerEventSeries.Points,
						[]interface{}{timestamp, re.At, re.Code, re.Desc, re.Method, re.Host, re.Path, re.RequestId, re.Fwd, re.Dyno, re.Connect, re.Service, re.Status, re.Bytes, re.Sock},
					)
					routerEventSeriesWithId.Points = append(
						routerEventSeriesWithId.Points,
						[]interface{}{timestamp, id, re.At, re.Code, re.Desc, re.Method, re.Host, re.Path, re.RequestId, re.Fwd, re.Dyno, re.Connect, re.Service, re.Status, re.Bytes, re.Sock},
					)

				// likely a standard router log
				default:
					ctx.Count("router.lines", 1)
					rm := routerMsg{}
					err := logfmt.Unmarshal(msg, &rm)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					routerSeries.Points = append(
						routerSeries.Points,
						[]interface{}{timestamp, rm.Bytes, rm.Status, rm.Service, rm.Connect, rm.Dyno, rm.Method, rm.Path, rm.Host, rm.RequestId, rm.Fwd},
					)
					routerSeriesWithId.Points = append(
						routerSeriesWithId.Points,
						[]interface{}{timestamp, id, rm.Bytes, rm.Status, rm.Service, rm.Connect, rm.Dyno, rm.Method, rm.Path, rm.Host, rm.RequestId, rm.Fwd},
					)
				}

				// Non router logs, so either dynos, runtime, etc
			default:
				switch {
				case bytes.HasPrefix(msg, dynoErrorSentinel):
					ctx.Count("dyno.error.lines", 1)
					de, err := parseBytesToDynoError(msg)
					if err != nil {
						log.Printf("Unable to parse dyno error message: %q\n", err)
					}
					dynoEvents.Points = append(
						dynoEvents.Points,
						[]interface{}{timestamp, string(lp.Header().Procid), "R", de.Code, string(msg)},
					)
					dynoEventsWithId.Points = append(
						dynoEventsWithId.Points,
						[]interface{}{timestamp, id, string(lp.Header().Procid), "R", de.Code, string(msg)},
					)

				case bytes.Contains(msg, dynoMemMsgSentinel):
					ctx.Count("dyno.mem.lines", 1)
					dm := dynoMemMsg{}
					err := logfmt.Unmarshal(msg, &dm)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					if dm.Source != "" {
						dynoMemSeries.Points = append(
							dynoMemSeries.Points,
							[]interface{}{timestamp, dm.Source, dm.MemoryCache, dm.MemoryPgpgin, dm.MemoryPgpgout, dm.MemoryRSS, dm.MemorySwap, dm.MemoryTotal},
						)
						dynoMemSeriesWithId.Points = append(
							dynoMemSeriesWithId.Points,
							[]interface{}{timestamp, id, dm.Source, dm.MemoryCache, dm.MemoryPgpgin, dm.MemoryPgpgout, dm.MemoryRSS, dm.MemorySwap, dm.MemoryTotal},
						)
					}
				case bytes.Contains(msg, dynoLoadMsgSentinel):
					ctx.Count("dyno.load.lines", 1)
					dm := dynoLoadMsg{}
					err := logfmt.Unmarshal(msg, &dm)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					if dm.Source != "" {
						dynoLoadSeries.Points = append(
							dynoLoadSeries.Points,
							[]interface{}{timestamp, dm.Source, dm.LoadAvg1Min, dm.LoadAvg5Min, dm.LoadAvg15Min},
						)
						dynoLoadSeriesWithId.Points = append(
							dynoLoadSeriesWithId.Points,
							[]interface{}{timestamp, id, dm.Source, dm.LoadAvg1Min, dm.LoadAvg5Min, dm.LoadAvg15Min},
						)
					}
				default: // unknown
					ctx.Count("unknown.heroku.lines", 1)
				}
			}
		default: // non heroku lines
			ctx.Count("non.heroku.lines", 1)
		}
	}
	ctx.MeasureSince("parse.time", parseStart)

	ctx.Count("router.points", len(routerSeries.Points))
	if len(routerSeries.Points) > 0 {
		routerSeries.Name = "router." + id
		routerSeries.Columns = []string{"time", "bytes", "status", "service", "connect", "dyno", "method", "path", "host", "requestId", "fwd"}
		series = append(series, routerSeries)
	}

	if len(routerSeriesWithId.Points) > 0 {
		routerSeriesWithId.Name = "router"
		routerSeriesWithId.Columns = []string{"time", "id", "bytes", "status", "service", "connect", "dyno", "method", "path", "host", "requestId", "fwd"}
		series = append(series, routerSeriesWithId)
	}

	ctx.Count("router.events.points", len(routerEventSeries.Points))
	if len(routerEventSeries.Points) > 0 {
		routerEventSeries.Name = "router.events." + id
		routerEventSeries.Columns = []string{"time", "at", "code", "desc", "method", "host", "path", "requestId", "fwd", "dyno", "connect", "service", "status", "bytes", "sock"}
		series = append(series, routerEventSeries)
	}
	if len(routerEventSeriesWithId.Points) > 0 {
		routerEventSeriesWithId.Name = "router.events"
		routerEventSeriesWithId.Columns = []string{"time", "id", "at", "code", "desc", "method", "host", "path", "requestId", "fwd", "dyno", "connect", "service", "status", "bytes", "sock"}
		series = append(series, routerEventSeriesWithId)
	}

	ctx.Count("dyno.mem.points", len(dynoMemSeries.Points))
	if len(dynoMemSeries.Points) > 0 {
		dynoMemSeries.Name = "dyno.mem." + id
		dynoMemSeries.Columns = []string{"time", "source", "memory_cache", "memory_pgpgin", "memory_pgpgout", "memory_rss", "memory_swap", "memory_total"}
		series = append(series, dynoMemSeries)
	}
	if len(dynoMemSeriesWithId.Points) > 0 {
		dynoMemSeriesWithId.Name = "dyno.mem"
		dynoMemSeriesWithId.Columns = []string{"time", "id", "source", "memory_cache", "memory_pgpgin", "memory_pgpgout", "memory_rss", "memory_swap", "memory_total"}
		series = append(series, dynoMemSeriesWithId)
	}

	ctx.Count("dyno.series.points", len(dynoLoadSeries.Points))
	if len(dynoLoadSeries.Points) > 0 {
		dynoLoadSeries.Name = "dyno.load." + id
		dynoLoadSeries.Columns = []string{"time", "source", "load_avg_1m", "load_avg_5m", "load_avg_15m"}
		series = append(series, dynoLoadSeries)
	}
	if len(dynoLoadSeriesWithId.Points) > 0 {
		dynoLoadSeriesWithId.Name = "dyno.load"
		dynoLoadSeriesWithId.Columns = []string{"time", "id", "source", "load_avg_1m", "load_avg_5m", "load_avg_15m"}
		series = append(series, dynoLoadSeriesWithId)
	}

	ctx.Count("dyno.events.points", len(dynoEvents.Points))
	if len(dynoEvents.Points) > 0 {
		dynoEvents.Name = "dyno.events." + id
		dynoEvents.Columns = []string{"time", "what", "type", "code", "message"}
		series = append(series, dynoEvents)
	}
	if len(dynoEventsWithId.Points) > 0 {
		dynoEventsWithId.Name = "dyno.events"
		dynoEventsWithId.Columns = []string{"time", "id", "what", "type", "code", "message"}
		series = append(series, dynoEventsWithId)
	}

	if len(series) > 0 {
		postStart := time.Now()
		err := influxClient.WriteSeriesWithTimePrecision(series, influx.Microsecond)
		if err != nil {
			log.Println(err)
		}
		ctx.MeasureSince("post.time", postStart)
	}

	w.WriteHeader(200)

	//data, err := json.Marshal(series)
	//if err != nil {
	//fmt.Println(err)
	//} else {
	//fmt.Println(string(data))
	//}

}
