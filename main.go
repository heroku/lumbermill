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
	"strconv"
	"strings"
	"time"

	"github.com/bmizerany/lpx"
	"github.com/go-martini/martini"
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

func serveDrain(w http.ResponseWriter, r *http.Request) {

	series := make([]*influx.Series, 0)
	routerSeries := &influx.Series{Points: make([][]interface{}, 0)}
	dynoMemSeries := &influx.Series{Points: make([][]interface{}, 0)}
	dynoLoadSeries := &influx.Series{Points: make([][]interface{}, 0)}

	//FIXME: Better auth? Encode the Token via Fernet and make that the user or password?
	id := r.Header.Get("Logplex-Drain-Token")
	log.Println("id: " + id)

	lp := lpx.NewReader(bufio.NewReader(r.Body))
	defer r.Body.Close()

	for lp.Next() {
		header := lp.Header()
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
				rm := routerMsg{}
				err := logfmt.Unmarshal(lp.Bytes(), &rm)
				if err != nil {
					log.Printf("logfmt unmarshal error: %s\n", err)
				} else {
					service, e := strconv.Atoi(strings.TrimSuffix(rm.Service, "ms"))
					if e != nil {
						log.Printf("Unable to Atoi on service time (%s): %s\n", rm.Service, e)
					}
					connect, e := strconv.Atoi(strings.TrimSuffix(rm.Connect, "ms"))
					if e != nil {
						log.Printf("Unable to Atoi on connect time (%s): %s\n", rm.Service, e)
					}
					routerSeries.Points = append(
						routerSeries.Points,
						[]interface{}{timestamp, rm.Bytes, rm.Status, service, connect, rm.Dyno, rm.Method, rm.Path, rm.Host, rm.RequestId, rm.Fwd},
					)
				}
			default:
				msg := lp.Bytes()
				switch {
				case bytes.Contains(msg, dynoMemMsgSentinal):
					dm := dynoMemMsg{}
					err := logfmt.Unmarshal(lp.Bytes(), &dm)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
					} else {
						if dm.Source != "" {
							dynoMemSeries.Points = append(
								dynoMemSeries.Points,
								[]interface{}{timestamp, dm.Source, dm.MemoryCache, dm.MemoryPgpgin, dm.MemoryPgpgout, dm.MemoryRSS, dm.MemorySwap, dm.MemoryTotal},
							)
						}
					}
				case bytes.Contains(msg, dynoLoadMsgSentinal):
					dm := dynoLoadMsg{}
					err := logfmt.Unmarshal(lp.Bytes(), &dm)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
					} else {
						if dm.Source != "" {
							dynoLoadSeries.Points = append(
								dynoLoadSeries.Points,
								[]interface{}{timestamp, dm.Source, dm.LoadAvg1Min, dm.LoadAvg5Min, dm.LoadAvg15Min},
							)
						}
					}

				}
			}
		}
	}

	if len(routerSeries.Points) > 0 {
		routerSeries.Name = "router." + id
		routerSeries.Columns = []string{"time", "bytes", "status", "service", "connect", "dyno", "method", "path", "host", "requestId", "fwd"}
		series = append(series, routerSeries)
	}

	if len(dynoMemSeries.Points) > 0 {
		dynoMemSeries.Name = "dyno.mem." + id
		dynoMemSeries.Columns = []string{"time", "source", "memory_cache", "memory_pgpgin", "memory_pgpgout", "memory_rss", "memory_swap", "memory_total"}
		series = append(series, dynoMemSeries)
	}
	if len(dynoLoadSeries.Points) > 0 {
		dynoLoadSeries.Name = "dyno.load." + id
		dynoLoadSeries.Columns = []string{"time", "source", "load_avg_1m", "load_avg_5m", "load_avg_15m"}
		series = append(series, dynoLoadSeries)
	}

	if len(series) > 0 {
		err := influxClient.WriteSeriesWithTimePrecision(series, influx.Microsecond)
		if err != nil {
			fmt.Println(err)
		}
	}

	w.WriteHeader(200)

	//data, err := json.Marshal(series)
	//if err != nil {
	//fmt.Println(err)
	//} else {
	//fmt.Println(string(data))
	//}

}

func main() {
	m := martini.Classic()
	m.Post("/drain", serveDrain)
	m.Run()
}
