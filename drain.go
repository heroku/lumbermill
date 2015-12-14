package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/bmizerany/lpx"
	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/kr/logfmt"
	metrics "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/rcrowley/go-metrics"
)

var (
	// TokenPrefix contains the prefix for non-heroku tokens.
	TokenPrefix = []byte("t.")
	// Heroku contains the prefix for heroku tokens.
	Heroku = []byte("heroku")

	debugToken = os.Getenv("DEBUG_TOKEN")

	// go-metrics Instruments
	wrongMethodErrorCounter    = metrics.GetOrRegisterCounter("lumbermill.errors.drain.wrong.method", metrics.DefaultRegistry)
	authFailureCounter         = metrics.GetOrRegisterCounter("lumbermill.errors.auth.failure", metrics.DefaultRegistry)
	badRequestCounter          = metrics.GetOrRegisterCounter("lumbermill.errors.badrequest", metrics.DefaultRegistry)
	internalServerErrorCounter = metrics.GetOrRegisterCounter("lumbermill.errors.internalserver", metrics.DefaultRegistry)
	tokenMissingCounter        = metrics.GetOrRegisterCounter("lumbermill.errors.token.missing", metrics.DefaultRegistry)
	timeParsingErrorCounter    = metrics.GetOrRegisterCounter("lumbermill.errors.time.parse", metrics.DefaultRegistry)
	logfmtParsingErrorCounter  = metrics.GetOrRegisterCounter("lumbermill.errors.logfmt.parse", metrics.DefaultRegistry)
	droppedErrorCounter        = metrics.GetOrRegisterCounter("lumbermill.errors.dropped", metrics.DefaultRegistry)
	batchCounter               = metrics.GetOrRegisterCounter("lumbermill.batch", metrics.DefaultRegistry)
	linesCounter               = metrics.GetOrRegisterCounter("lumbermill.lines", metrics.DefaultRegistry)
	routerErrorLinesCounter    = metrics.GetOrRegisterCounter("lumbermill.lines.router.error", metrics.DefaultRegistry)
	routerLinesCounter         = metrics.GetOrRegisterCounter("lumbermill.lines.router", metrics.DefaultRegistry)
	routerBlankLinesCounter    = metrics.GetOrRegisterCounter("lumbermill.lines.router.blank", metrics.DefaultRegistry)
	dynoErrorLinesCounter      = metrics.GetOrRegisterCounter("lumbermill.lines.dyno.error", metrics.DefaultRegistry)
	dynoMemLinesCounter        = metrics.GetOrRegisterCounter("lumbermill.lines.dyno.mem", metrics.DefaultRegistry)
	dynoLoadLinesCounter       = metrics.GetOrRegisterCounter("lumbermill.lines.dyno.load", metrics.DefaultRegistry)
	unknownHerokuLinesCounter  = metrics.GetOrRegisterCounter("lumbermill.lines.unknown.heroku", metrics.DefaultRegistry)
	unknownUserLinesCounter    = metrics.GetOrRegisterCounter("lumbermill.lines.unknown.user", metrics.DefaultRegistry)
	parseTimer                 = metrics.GetOrRegisterTimer("lumbermill.batches.parse.time", metrics.DefaultRegistry)
	batchSizeHistogram         = metrics.GetOrRegisterHistogram("lumbermill.batches.sizes", metrics.DefaultRegistry, metrics.NewUniformSample(100))

	// The other firehoses we'll want to shove the proxy to.
	shadowURLs      = make(map[string]*big.Int)
	shadowPostError = metrics.GetOrRegisterCounter("lumbermill.errors.shadow.post", metrics.DefaultRegistry)
	shadowMutex     = sync.RWMutex{}
)

func init() {
	if err := setShadowURLs(strings.Split(os.Getenv("SHADOW_URLS"), ",")); err != nil {
		log.Println(err)
	}
}

func setShadowURLs(urls []string) (err error) {
	shadowMutex.Lock()
	defer shadowMutex.Unlock()

	for _, u := range urls {
		u, e := url.Parse(u)
		if e != nil {
			err = e
			continue
		}

		// We parse the fragment as a percentage of traffic, e.g. #5 == 5%
		percentage, e := strconv.Atoi(u.Fragment)
		if e != nil {
			err = fmt.Errorf("!! shadowURL: Unable to parse fragment = %s, assuming 100%", u.Fragment)
			percentage = 100
		}

		// Clear the fragement for posting.
		u.Fragment = ""

		shadowURLs[u.String()] = big.NewInt(int64(percentage))

		u.User = nil
		log.Printf("==> Successfully set %s to %d percent", u.String(), percentage)
	}

	return err
}

// Dyno's are generally reported as "<type>.<#>"
// Extract the <type> and return it
func dynoType(what string) string {
	s := strings.Split(what, ".")
	return s[0]
}

// Lock, or don't do any work, but don't block.
// This, essentially, samples the incoming tokens for the purposes of health checking
// live tokens. Rather than use a random number generator, or a global counter, we
// let the scheduler do the sampling for us.
func (s *server) maybeUpdateRecentTokens(host, id string) {
	if atomic.CompareAndSwapInt32(s.tokenLock, 0, 1) {
		s.recentTokensLock.Lock()
		s.recentTokens[host] = id
		s.recentTokensLock.Unlock()
		atomic.StoreInt32(s.tokenLock, 0)
	}
}

func handleLogFmtParsingError(msg []byte, err error) {
	logfmtParsingErrorCounter.Inc(1)
	log.Printf("logfmt unmarshal error(%q): %q\n", string(msg), err)
}

// "Parse tree" from hell
func (s *server) serveDrain(w http.ResponseWriter, r *http.Request) {
	s.Add(1)
	defer s.Done()

	w.Header().Set("Content-Length", "0")

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		wrongMethodErrorCounter.Inc(1)
		return
	}

	id := r.Header.Get("Logplex-Drain-Token")

	batchCounter.Inc(1)

	parseStart := time.Now()
	buf := bytes.Buffer{}
	lp := lpx.NewReader(bufio.NewReader(io.TeeReader(r.Body, &buf)))

	linesCounterInc := 0

	for lp.Next() {
		linesCounterInc++
		header := lp.Header()

		// If the syslog Name Header field contains what looks like a log token,
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

		destination := s.hashRing.Get(id)

		msg := lp.Bytes()
		switch {
		case bytes.Equal(header.Name, Heroku), bytes.HasPrefix(header.Name, TokenPrefix):
			timeStr := string(lp.Header().Time)
			t, e := time.Parse("2006-01-02T15:04:05.000000+00:00", timeStr)
			if e != nil {
				t, e = time.Parse("2006-01-02T15:04:05+00:00", timeStr)
				if e != nil {
					timeParsingErrorCounter.Inc(1)
					log.Printf("Error Parsing Time(%s): %q\n", string(lp.Header().Time), e)
					continue
				}
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
						handleLogFmtParsingError(msg, err)
						continue
					}

					// Track the breakout of different error types.
					metrics.GetOrRegisterCounter("lumbermill.lines.router.errors."+re.Code, metrics.DefaultRegistry).Inc(1)

					if debugToken != "" && id == debugToken {
						log.Printf("debug=error.%s %s", re.Code, msg)
					}

					destination.PostPoint(point{id, routerEvent, []interface{}{timestamp, re.Code}})

					// If the app is blank (not pushed) we don't care
				// do nothing atm, increment a counter
				case bytes.Contains(msg, keyCodeBlank), bytes.Contains(msg, keyDescBlank):
					routerBlankLinesCounter.Inc(1)

				// likely a standard router log
				default:
					routerLinesCounter.Inc(1)
					rm := routerMsg{}
					err := logfmt.Unmarshal(msg, &rm)
					if err != nil {
						handleLogFmtParsingError(msg, err)
						continue
					}

					destination.PostPoint(point{id, routerRequest, []interface{}{timestamp, rm.Status, rm.Service}})
				}

				// Non router logs, so either dynos, runtime, etc
			default:
				switch {
				// Dyno error messages
				case bytes.HasPrefix(msg, dynoErrorSentinel):
					dynoErrorLinesCounter.Inc(1)
					de, err := parseBytesToDynoError(msg)
					if err != nil {
						handleLogFmtParsingError(msg, err)
						continue
					}

					what := string(lp.Header().Procid)
					destination.PostPoint(
						point{id, dynoEvents, []interface{}{timestamp, what, "R", de.Code, string(msg), dynoType(what)}},
					)

				// Dyno log-runtime-metrics memory messages
				case bytes.Contains(msg, dynoMemMsgSentinel):
					s.maybeUpdateRecentTokens(destination.Name, id)

					dynoMemLinesCounter.Inc(1)
					dm := dynoMemMsg{}
					err := logfmt.Unmarshal(msg, &dm)
					if err != nil {
						handleLogFmtParsingError(msg, err)
						continue
					}
					if dm.Source != "" {
						destination.PostPoint(
							point{
								id,
								dynoMem,
								[]interface{}{
									timestamp,
									dm.Source,
									dm.MemoryCache,
									dm.MemoryPgpgin,
									dm.MemoryPgpgout,
									dm.MemoryRSS,
									dm.MemorySwap,
									dm.MemoryTotal,
									dynoType(dm.Source),
								},
							},
						)
					}

					// Dyno log-runtime-metrics load messages
				case bytes.Contains(msg, dynoLoadMsgSentinel):
					s.maybeUpdateRecentTokens(destination.Name, id)

					dynoLoadLinesCounter.Inc(1)
					dm := dynoLoadMsg{}
					err := logfmt.Unmarshal(msg, &dm)
					if err != nil {
						handleLogFmtParsingError(msg, err)
						continue
					}
					if dm.Source != "" {
						destination.PostPoint(
							point{
								id,
								dynoLoad,
								[]interface{}{timestamp, dm.Source, dm.LoadAvg1Min, dm.LoadAvg5Min, dm.LoadAvg15Min, dynoType(dm.Source)},
							},
						)
					}

				// unknown
				default:
					unknownHerokuLinesCounter.Inc(1)
					if debug {
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
			if debug {
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

	linesCounter.Inc(int64(linesCounterInc))

	batchSizeHistogram.Update(int64(linesCounterInc))

	parseTimer.UpdateSince(parseStart)

	if len(shadowURLs) > 0 {
		go amplify(r.Header, buf)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) serveShadowURLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var urls []string
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&urls); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := setShadowURLs(urls); err != nil {
		log.Println(err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func getShadowURLs() map[string]*big.Int {
	shadowMutex.RLock()
	urls := shadowURLs
	shadowMutex.RUnlock()
	return urls
}

func amplify(headers http.Header, buf bytes.Buffer) {
	for url, perc := range getShadowURLs() {
		if !balance(perc) {
			continue
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(buf.Bytes()))
		if err != nil {
			shadowPostError.Inc(1)
			continue
		}

		for key, values := range headers {
			// Blacklist authorization header
			if strings.ToUpper(key) == "AUTHORIZATION" {
				continue
			}

			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			shadowPostError.Inc(1)
			continue
		}
		resp.Body.Close()
	}
}

// Optimization to avoid creating a new big.Int var for every request.
var oneHundred = big.NewInt(100)

func balance(perc *big.Int) bool {
	n, _ := rand.Int(rand.Reader, oneHundred)
	return n.Cmp(perc) == -1
}
