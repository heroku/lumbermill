package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bmizerany/lpx"
	"github.com/kr/logfmt"
	metrics "github.com/rcrowley/go-metrics"
)

var (
	TokenPrefix = []byte("t.")
	Heroku      = []byte("heroku")

	// go-metrics Instruments
	wrongMethodErrorCounter   = metrics.NewRegisteredCounter("lumbermill.errors.drain.wrong.method", metrics.DefaultRegistry)
	authFailureCounter        = metrics.NewRegisteredCounter("lumbermill.errors.auth.failure", metrics.DefaultRegistry)
	tokenMissingCounter       = metrics.NewRegisteredCounter("lumbermill.errors.token.missing", metrics.DefaultRegistry)
	timeParsingErrorCounter   = metrics.NewRegisteredCounter("lumbermill.errors.time.parse", metrics.DefaultRegistry)
	logfmtParsingErrorCounter = metrics.NewRegisteredCounter("lumbermill.errors.logfmt.parse", metrics.DefaultRegistry)
	batchCounter              = metrics.NewRegisteredCounter("lumbermill.batch", metrics.DefaultRegistry)
	linesCounter              = metrics.NewRegisteredCounter("lumbermill.lines", metrics.DefaultRegistry)
	routerErrorLinesCounter   = metrics.NewRegisteredCounter("lumbermill.lines.router.error", metrics.DefaultRegistry)
	routerLinesCounter        = metrics.NewRegisteredCounter("lumbermill.lines.router", metrics.DefaultRegistry)
	dynoErrorLinesCounter     = metrics.NewRegisteredCounter("lumbermill.lines.dyno.error", metrics.DefaultRegistry)
	dynoMemLinesCounter       = metrics.NewRegisteredCounter("lumbermill.lines.dyno.mem", metrics.DefaultRegistry)
	dynoLoadLinesCounter      = metrics.NewRegisteredCounter("lumbermill.lines.dyno.load", metrics.DefaultRegistry)
	unknownHerokuLinesCounter = metrics.NewRegisteredCounter("lumbermill.lines.unknown.heroku", metrics.DefaultRegistry)
	unknownUserLinesCounter   = metrics.NewRegisteredCounter("lumbermill.lines.unknown.user", metrics.DefaultRegistry)
	parseTimer                = metrics.NewRegisteredTimer("lumbermill.batches.parse.time", metrics.DefaultRegistry)
)

func checkAuth(r *http.Request) error {
	header := r.Header.Get("Authorization")
	if header == "" {
		return errors.New("Authorization required")
	}
	headerParts := strings.SplitN(header, " ", 2)
	if len(headerParts) != 2 {
		return errors.New("Authorization header is malformed")
	}

	method := headerParts[0]
	if method != "Basic" {
		return errors.New("Only Basic Authorization is accepted")
	}

	encodedUserPass := headerParts[1]
	decodedUserPass, err := base64.StdEncoding.DecodeString(encodedUserPass)
	if err != nil {
		return errors.New("Authorization header is malformed")
	}

	userPassParts := bytes.SplitN(decodedUserPass, []byte{':'}, 2)
	if len(userPassParts) != 2 {
		return errors.New("Authorization header is malformed")
	}

	user := userPassParts[0]
	pass := userPassParts[1]

	if string(user) != User {
		return errors.New("Unknown user")
	}
	if string(pass) != Password {
		return errors.New("Incorrect token")
	}

	return nil
}

// Dyno's are generally reported as "<type>.<#>"
// Extract the <type> and return it
func dynoType(what string) string {
	s := strings.Split(what, ".")
	return s[0]
}

// "Parse tree" from hell
func serveDrain(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Length", "0")

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		wrongMethodErrorCounter.Inc(1)
		return
	}

	id := r.Header.Get("Logplex-Drain-Token")

	if id == "" {
		if err := checkAuth(r); err != nil {
			w.WriteHeader(http.StatusForbidden)
			authFailureCounter.Inc(1)
			return
		}
	}

	batchCounter.Inc(1)

	parseStart := time.Now()
	lp := lpx.NewReader(bufio.NewReader(r.Body))

	for lp.Next() {
		linesCounter.Inc(1)
		header := lp.Header()

		// If the syslog App Name Header field containts what looks like a log token,
		// let's assume it's an override of the id and we're getting the data from the magic
		// channel
		if bytes.HasPrefix(header.Name, TokenPrefix) {
			id = string(header.Name)
		}

		// If we still don't have an id, throw an error and try the next line
		if id == "" {
			tokenMissingCounter.Inc(1)
			continue
		}

		chanGroup := hashRing.Get(id)

		msg := lp.Bytes()
		switch {
		case bytes.Equal(header.Name, Heroku), bytes.HasPrefix(header.Name, TokenPrefix):
			t, e := time.Parse("2006-01-02T15:04:05.000000+00:00", string(lp.Header().Time))
			if e != nil {
				timeParsingErrorCounter.Inc(1)
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
					routerErrorLinesCounter.Inc(1)
					re := routerError{}
					err := logfmt.Unmarshal(msg, &re)
					if err != nil {
						logfmtParsingErrorCounter.Inc(1)
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					chanGroup.points[EventsRouter] <- []interface{}{timestamp, id, re.Code}

				// likely a standard router log
				default:
					routerLinesCounter.Inc(1)
					rm := routerMsg{}
					err := logfmt.Unmarshal(msg, &rm)
					if err != nil {
						logfmtParsingErrorCounter.Inc(1)
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					chanGroup.points[Router] <- []interface{}{timestamp, id, rm.Status, rm.Service}
				}

				// Non router logs, so either dynos, runtime, etc
			default:
				switch {
				// Dyno error messages
				case bytes.HasPrefix(msg, dynoErrorSentinel):
					dynoErrorLinesCounter.Inc(1)
					de, err := parseBytesToDynoError(msg)
					if err != nil {
						log.Printf("Unable to parse dyno error message: %q\n", err)
					}

					what := string(lp.Header().Procid)
					chanGroup.points[EventsDyno] <- []interface{}{
						timestamp,
						id,
						what,
						"R",
						de.Code,
						string(msg),
						dynoType(what),
					}

				// Dyno log-runtime-metrics memory messages
				case bytes.Contains(msg, dynoMemMsgSentinel):
					dynoMemLinesCounter.Inc(1)
					dm := dynoMemMsg{}
					err := logfmt.Unmarshal(msg, &dm)
					if err != nil {
						logfmtParsingErrorCounter.Inc(1)
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					if dm.Source != "" {
						chanGroup.points[DynoMem] <- []interface{}{
							timestamp,
							id,
							dm.Source,
							dm.MemoryCache,
							dm.MemoryPgpgin,
							dm.MemoryPgpgout,
							dm.MemoryRSS,
							dm.MemorySwap,
							dm.MemoryTotal,
							dynoType(dm.Source),
						}
					}

					// Dyno log-runtime-metrics load messages
				case bytes.Contains(msg, dynoLoadMsgSentinel):
					dynoLoadLinesCounter.Inc(1)
					dm := dynoLoadMsg{}
					err := logfmt.Unmarshal(msg, &dm)
					if err != nil {
						logfmtParsingErrorCounter.Inc(1)
						log.Printf("logfmt unmarshal error: %s\n", err)
						continue
					}
					if dm.Source != "" {
						chanGroup.points[DynoLoad] <- []interface{}{
							timestamp,
							id,
							dm.Source,
							dm.LoadAvg1Min,
							dm.LoadAvg5Min,
							dm.LoadAvg15Min,
							dynoType(dm.Source),
						}
					}

				// unknown
				default:
					unknownHerokuLinesCounter.Inc(1)
					if Debug {
						log.Printf("Unknown Heroku Line - Header: PRI: %s, Time: %s, Hostname: %s, Name: %s, ProcId: %s, MsgId: %s - Body: %s",
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
			}

		// non heroku lines
		default:
			unknownUserLinesCounter.Inc(1)
			if Debug {
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
	}

	parseTimer.UpdateSince(parseStart)

	// If we are told to close the connection after the reply, do so.
	select {
	case <-connectionCloser:
		w.Header().Set("Connection", "close")
	default:
		//Nothing
	}

	w.WriteHeader(http.StatusNoContent)
}
