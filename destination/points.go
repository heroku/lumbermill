package destination

// Encodes eries Type information
type seriesType int

const (
	routerRequest seriesType = iota
	routerEvent
	dynoMem
	dynoLoad
	dynoEvents
	numSeries
)

var (
	seriesColumns = [][]string{
		[]string{"time", "status", "service"}, // Router
		[]string{"time", "code"},              // EventsRouter
		[]string{"time", "source", "memory_cache", "memory_pgpgin", "memory_pgpgout", "memory_rss", "memory_swap", "memory_total", "dynoType"}, // DynoMem
		[]string{"time", "source", "load_avg_1m", "load_avg_5m", "load_avg_15m", "dynoType"},                                                   // DynoLoad
		[]string{"time", "what", "type", "code", "message", "dynoType"},                                                                        // DynoEvents
	}

	seriesNames = []string{"router", "events.router", "dyno.mem", "dyno.load", "events.dyno"}
)

func (st seriesType) Name() string {
	return seriesNames[st]
}

func (st seriesType) Columns() []string {
	return seriesColumns[st]
}

// Holds data around a data point
type point struct {
	Token  string
	Type   seriesType
	Points []interface{}
}

func (p point) SeriesName() string {
	return p.Type.Name() + "." + p.Token
}
