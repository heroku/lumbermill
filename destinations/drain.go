package destinations

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/bmizerany/lpx"
	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/heroku/logma"
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
)

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
	defer parseTimer.UpdateSince(time.Now())

	lp := lpx.NewReader(bufio.NewReader(r.Body))
	linesCounterInc := 0

	for lp.Next() {
		linesCounterInc++
		envelope, err := logma.LpxToEnvelope(lp, id)
		if err != nil {
			continue
		}

		// If we still don't have an id, throw an error and try the next line
		if envelope.Owner == "" {
			tokenMissingCounter.Inc(1)
			continue
		}

		destination := s.hashRing.Get(envelope.Owner)

		switch envelope.Type {
		case "RouterError":
			routerErrorLinesCounter.Inc(1)
			destination.Post(envelope)

			// TODO: figure out if we need this or not
			// Track the breakout of different error types.
			re := envelope.Value.(*logma.RouterError)
			metrics.GetOrRegisterCounter("lumbermill.lines.router.errors."+re.Code, metrics.DefaultRegistry).Inc(1)

		case "RouterRequest":
			routerLinesCounter.Inc(1)
			destination.Post(envelope)

		default:
			log.Printf("DEFAULT")
		}
	}

	linesCounter.Inc(int64(linesCounterInc))
	batchSizeHistogram.Update(int64(linesCounterInc))

	w.WriteHeader(http.StatusNoContent)
}
