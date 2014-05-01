package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/bmizerany/lpx"
	"github.com/go-martini/martini"
	influx "github.com/influxdb/influxdb-go"
	"github.com/kr/logfmt"
)

type routerMsg struct {
	Bytes     int
	Status    int
	Service   string
	Connect   string
	Dyno      string
	Method    string
	Path      string
	Host      string
	RequestId string
	Fwd       string
}

var (
	influxClientConfig influx.ClientConfig
	influxClient       *influx.Client
)

func init() {
	var err error

	influxClientConfig = influx.ClientConfig{
		Host:     "influxor.ssl.edward.herokudev.com:8086",
		Username: "test",
		Password: "tester",
		Database: "ingress",
		IsSecure: true,
		HttpClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
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

	//FIXME: Better auth? Encode the Token via Fernet and make that the user or password?
	id := r.Header.Get("Logplex-Drain-Token")
	log.Println("id: " + id)

	lp := lpx.NewReader(bufio.NewReader(r.Body))
	for lp.Next() {
		switch string(lp.Header().Name) {
		case "router":
			rm := routerMsg{}
			err := logfmt.Unmarshal(lp.Bytes(), &rm)
			if err != nil {
				log.Printf("logfmt unmarshal error: %s\n", err)
			} else {
				t, e := time.Parse("2006-01-02T15:04:05.000000+00:00", string(lp.Header().Time))
				if e != nil {
					log.Printf("Error Parsing Time(%s): %q\n", string(lp.Header().Time), e)
				} else {
					routerSeries.Points = append(
						routerSeries.Points,
						[]interface{}{t.UnixNano() / int64(time.Millisecond), rm.Bytes, rm.Status, rm.Service, rm.Connect, rm.Dyno, rm.Method, rm.Path, rm.Host, rm.RequestId, rm.Fwd},
					)
				}
			}
		default:
			log.Printf("other: %+v\n", lp.Header())
		}
	}

	if len(routerSeries.Points) > 0 {
		routerSeries.Name = "router." + id
		routerSeries.Columns = []string{"time", "bytes", "status", "service", "connect", "dyno", "method", "path", "host", "requestId", "fwd"}
		series = append(series, routerSeries)

		err := influxClient.WriteSeries(series)
		if err != nil {
			fmt.Println(err)
		}
	}

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
