package main

import (
	"bufio"
	"bytes"
	"log"
	"net/http"
	"time"

	"github.com/bmizerany/lpx"
	"github.com/heroku/slog"
	"github.com/kr/logfmt"
)

var (
	TokenPrefix = []byte("t.")
	Heroku      = []byte("heroku")
)

// "Parse tree" from hell
func serveDrain(w http.ResponseWriter, r *http.Request) {
	ctx := slog.Context{}
	defer func() { LogWithContext(ctx) }()

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		ctx.Count("errors.drain.wrong.method", 1)
		return
	}

	id := r.Header.Get("Logplex-Drain-Token")

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		ctx.Count("errors.drain.missing.token", 1)
		return
	}

	ctx.Count("batch", 1)

	parseStart := time.Now()
	lp := lpx.NewReader(bufio.NewReader(r.Body))
	defer r.Body.Close()

	for lp.Next() {
		ctx.Count("lines.total", 1)
		header := lp.Header()

		// If the syslog App Name Header field containts what looks like a log token,
		// let's assume it's an override of the id and we're getting the data from the magic
		// channel
		if bytes.HasPrefix(header.Name, TokenPrefix) {
			id = string(header.Name)
		}

		msg := lp.Bytes()
		switch {
		case bytes.Equal(header.Name, Heroku), bytes.HasPrefix(header.Name, TokenPrefix):
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
					ctx.Count("lines.router.error", 1)
					re := routerError{}
					err := logfmt.Unmarshal(msg, &re)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					routerEventPoints <- []interface{}{timestamp, id, re.At, re.Code, re.Desc, re.Method, re.Host, re.Path, re.RequestId, re.Fwd, re.Dyno, re.Connect, re.Service, re.Status, re.Bytes, re.Sock}

				// likely a standard router log
				default:
					ctx.Count("lines.router", 1)
					rm := routerMsg{}
					err := logfmt.Unmarshal(msg, &rm)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					routerPoints <- []interface{}{timestamp, id, rm.Bytes, rm.Status, rm.Service, rm.Connect, rm.Dyno, rm.Method, rm.Path, rm.Host, rm.RequestId, rm.Fwd}
				}

				// Non router logs, so either dynos, runtime, etc
			default:
				switch {
				case bytes.HasPrefix(msg, dynoErrorSentinel):
					ctx.Count("lines.dyno.error", 1)
					de, err := parseBytesToDynoError(msg)
					if err != nil {
						log.Printf("Unable to parse dyno error message: %q\n", err)
					}
					dynoEventsPoints <- []interface{}{timestamp, id, string(lp.Header().Procid), "R", de.Code, string(msg)}

				case bytes.Contains(msg, dynoMemMsgSentinel):
					ctx.Count("lines.dyno.mem", 1)
					dm := dynoMemMsg{}
					err := logfmt.Unmarshal(msg, &dm)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					if dm.Source != "" {
						dynoMemPoints <- []interface{}{timestamp, id, dm.Source, dm.MemoryCache, dm.MemoryPgpgin, dm.MemoryPgpgout, dm.MemoryRSS, dm.MemorySwap, dm.MemoryTotal}
					}
				case bytes.Contains(msg, dynoLoadMsgSentinel):
					ctx.Count("lines.dyno.load", 1)
					dm := dynoLoadMsg{}
					err := logfmt.Unmarshal(msg, &dm)
					if err != nil {
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					if dm.Source != "" {
						dynoLoadPoints <- []interface{}{timestamp, id, dm.Source, dm.LoadAvg1Min, dm.LoadAvg5Min, dm.LoadAvg15Min}
					}
				default: // unknown
					ctx.Count("lines.unknown.heroku", 1)
				}
			}
		default: // non heroku lines
			ctx.Count("lines.unknown.user", 1)
			log.Printf("Unknown User Line - Header: PRI: %s, Time: %s, Hostname: %s, Name: %s, ProcId: %s, MsgId: %s - Body: %s",
				header.PrivalVersion,
				header.Time,
				header.Hostname,
				header.Name,
				header.Procid,
				header.Msgid,
				string(msg),
			)
		}
	}
	ctx.MeasureSince("lines.parse.time", parseStart)

	w.WriteHeader(http.StatusOK)
}
