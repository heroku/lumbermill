package lpxgen

import (
	"fmt"
	"math/rand"
	"time"
)

type HerokuLog int

const (
	Router HerokuLog = iota
	DynoMem
	DynoLoad
	numHerokuLog
)

var (
	paths = []string{
		"/about", "/api", "/blog", "/docs", "/events", "/help", "/legal",
		"/policy", "/pricing", "/privacy", "/security", "/support", "/tos",
	}
	methods  = []string{"GET", "HEAD", "POST"}
	statuses = []string{"200", "301", "302", "400", "401", "403", "404", "500"}
)

func (d HerokuLog) PrivalVersion() string {
	switch d {
	case Router:
		return "<158>1"
	case DynoMem, DynoLoad:
		return "<150>1"
	}
	return "<178>1"
}

func (d HerokuLog) Time() string {
	return time.Now().UTC().Format(TimeFormat)
}

func (d HerokuLog) Hostname() string {
	return "host"
}

func (d HerokuLog) Name() string {
	switch d {
	case Router:
		return RandomDrainToken()
	case DynoMem:
		return RandomDrainToken()
	case DynoLoad:
		return RandomDrainToken()
	}
	return "localhost"
}

func (d HerokuLog) Procid() string {
	switch d {
	case Router:
		return "router"
	case DynoMem:
		return "dynomem"
	case DynoLoad:
		return "dynoload"
	}
	return "app"
}

func (d HerokuLog) Msgid() string {
	return "-"
}

func (d HerokuLog) Msg() string {
	switch d {
	case Router:
		return fmt.Sprintf(`at=info method=%s path="%s" host=%s.herokuapp.com request_id=%s fwd="%s" dyno=web.1 connect=%dms service=%dms status=%s bytes=%d`,
			methods[rand.Intn(len(methods))],
			paths[rand.Intn(len(paths))],
			randomHexString(8),
			UUID4(),
			RandomIPv4(),
			rand.Intn(100),
			rand.Intn(600),
			statuses[rand.Intn(len(statuses))],
			300+rand.Intn(1000))
	case DynoLoad:
		return fmt.Sprintf(`source=web.%d dyno=heroku.%d.%s sample#load_avg_1m=%0.2f sample#load_avg_5m=%0.2f sample#load_avg_15m=%0.2f`,
			rand.Intn(5),
			rand.Intn(1000000),
			UUID4(),
			rand.Float32()*5.0,
			rand.Float32()*5.0,
			rand.Float32()*5.0)
	case DynoMem:
		return fmt.Sprintf(`source=web.%d dyno=heroku.%d.%s sample#memory_total=%0.2fMB sample#memory_rss=%0.2fMB sample#memory_cache=%0.2fMB sample#memory_swap=%0.2fMB sample#memory_pgpgin=%dpages sample#memory_pgpgout=%dpages`,
			rand.Intn(5),
			rand.Intn(1000000),
			UUID4(),
			rand.Float32()*512.0,
			rand.Float32()*256.0,
			rand.Float32()*0.01,
			rand.Float32()*0.01,
			rand.Intn(400000),
			rand.Intn(400000))
	}

	return fmt.Sprintf("invalid number: %f is not an integer", rand.Float32())
}

func (d HerokuLog) String() string {
	s := FormatSyslog(d)
	return fmt.Sprintf("%d %s", len(s), s)
}
