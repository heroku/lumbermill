package destinations

import "github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/heroku/logma"

func envelopeToPoint(envelope *logma.Envelope) point {
	switch envelope.Type {
	case "RouterError":
		re := envelope.Value.(*logma.RouterError)
		return point{envelope.Owner, routerEvent, []interface{}{envelope.Time, re.Code}}

	case "RouterRequest":
		rr := envelope.Value.(*logma.RouterRequest)
		return point{envelope.Owner, routerRequest, []interface{}{envelope.Time, rr.Status, rr.Service}}

	case "DynoError":
		de := envelope.Value.(*logma.DynoError)
		what := ""        // Should be Procid
		msg := []byte("") // Should be lp.Bytes (or entire log line)
		return point{
			envelope.Owner,
			dynoEvents,
			[]interface{}{
				envelope.Time,
				what, "R",
				de.Code,
				string(msg),
				dynoType(what),
			},
		}

	case "DynoLoad":
		dm := envelope.Value.(*logma.DynoLoad)

		return point{
			envelope.Owner,
			dynoLoad,
			[]interface{}{
				envelope.Time,
				dm.Source,
				dm.LoadAvg1Min,
				dm.LoadAvg5Min,
				dm.LoadAvg15Min,
				dynoType(dm.Source),
			},
		}

	case "DynoMemory":
		dm := envelope.Value.(*logma.DynoMemory)

		return point{
			envelope.Owner,
			dynoMem,
			[]interface{}{
				envelope.Time,
				dm.Source,
				dm.MemoryCache,
				dm.MemoryPgpgin,
				dm.MemoryPgpgout,
				dm.MemoryRSS,
				dm.MemorySwap,
				dm.MemoryTotal,
				dynoType(dm.Source),
			},
		}

	default:
		panic("Unknown type: " + envelope.Type)
	}
}
