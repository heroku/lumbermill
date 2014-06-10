package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/heroku/slog"
	influx "github.com/influxdb/influxdb-go"
)

const (
	PointChannelCapacity = 100000
)

var (
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

	routerPoints      = make(chan []interface{}, PointChannelCapacity)
	routerEventPoints = make(chan []interface{}, PointChannelCapacity)
	dynoMemPoints     = make(chan []interface{}, PointChannelCapacity)
	dynoLoadPoints    = make(chan []interface{}, PointChannelCapacity)
	dynoEventsPoints  = make(chan []interface{}, PointChannelCapacity)

	posters = make([]*Poster, 0)

	routerColumns      = []string{"time", "id", "bytes", "status", "service", "connect", "dyno", "method", "path", "host", "requestId", "fwd"}
	routerEventColumns = []string{"time", "id", "at", "code", "desc", "method", "host", "path", "requestId", "fwd", "dyno", "connect", "service", "status", "bytes", "sock"}
	dynoMemColumns     = []string{"time", "id", "source", "memory_cache", "memory_pgpgin", "memory_pgpgout", "memory_rss", "memory_swap", "memory_total"}
	dynoLoadColumns    = []string{"time", "id", "source", "load_avg_1m", "load_avg_5m", "load_avg_15m"}
	dynoEventsColumns  = []string{"time", "id", "what", "type", "code", "message"}
)

func LogWithContext(ctx slog.Context) {
	ctx.Add("app", "lumbermill")
	log.Println(ctx)
}

func main() {
	port := os.Getenv("PORT")

	posters = append(posters, NewPoster(influxClientConfig, "router", routerPoints, routerColumns))
	posters = append(posters, NewPoster(influxClientConfig, "router", routerPoints, routerColumns))
	posters = append(posters, NewPoster(influxClientConfig, "router", routerPoints, routerColumns))
	posters = append(posters, NewPoster(influxClientConfig, "router", routerPoints, routerColumns))
	posters = append(posters, NewPoster(influxClientConfig, "router", routerPoints, routerColumns))
	posters = append(posters, NewPoster(influxClientConfig, "events.router", routerEventPoints, routerEventColumns))
	posters = append(posters, NewPoster(influxClientConfig, "dyno.mem", dynoMemPoints, dynoMemColumns))
	posters = append(posters, NewPoster(influxClientConfig, "dyno.load", dynoLoadPoints, dynoLoadColumns))
	posters = append(posters, NewPoster(influxClientConfig, "events.dyno", dynoEventsPoints, dynoEventsColumns))

	for _, poster := range posters {
		go poster.Run()
	}

	// Some statistics about the channels this way we can see how full they are getting
	go func() {
		for {
			ctx := slog.Context{}
			time.Sleep(10 * time.Second)
			ctx.Sample("points.router.pending", len(routerPoints))
			ctx.Sample("points.events.router.pending", len(routerEventPoints))
			ctx.Sample("points.dyno.mem.pending", len(dynoMemPoints))
			ctx.Sample("points.dyno.load.pending", len(dynoLoadPoints))
			ctx.Sample("points.evetns.dyno.pending", len(dynoEventsPoints))
			LogWithContext(ctx)
		}
	}()

	http.HandleFunc("/drain", serveDrain)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
