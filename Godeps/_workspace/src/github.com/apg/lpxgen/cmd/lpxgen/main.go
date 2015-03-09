package main

import (
	"flag"
	"fmt"
	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/apg/lpxgen"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	count    = flag.Int("count", 1000, "Number of batches to send")
	minbatch = flag.Int("min", 1, "Minimum number of messages in a batch")
	maxbatch = flag.Int("max", 100, "Maximum number of messages in a batch")
	logdist  = flag.String("dist", "default", "Distribution of log types. <type>:0.9,<type>:0.1")
)

func logFromString(l string) lpxgen.Log {
	switch l {
	case "router":
		return lpxgen.Router
	case "dynomem":
		return lpxgen.DynoMem
	case "dynoload":
		return lpxgen.DynoLoad
	case "default":
		return lpxgen.DefaultLog{}
	default:
		fmt.Fprintf(os.Stderr, "WARNING: Invalid logtype %q: returning DefaultLog\n", l)
		return lpxgen.DefaultLog{}
	}
}

func parseDist(ds string) lpxgen.Log {
	logs := make(map[lpxgen.Log]float32)

	bits := strings.Split(ds, ",")
	for _, bit := range bits {
		logspec := strings.Split(bit, ":")
		switch len(logspec) {
		case 2:
			if val, err := strconv.ParseFloat(logspec[1], 32); err != nil {
				fmt.Printf("Invalid log spec: %q is not a valid number\n", logspec[1])
				os.Exit(1)
			} else {
				logs[logFromString(logspec[0])] = float32(val)
			}
		case 1:
			logs[logFromString(logspec[0])] = float32(1.0 / len(bits))
		default:
			fmt.Printf("Invalid log spec: %q\n", bit)
			os.Exit(1)
		}
	}

	return lpxgen.NewProbLog(logs)
}

func main() {
	flag.Parse()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [options] URL\n", os.Args[0])
		flag.PrintDefaults()
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "ERROR: No URL given\n\n")
		flag.Usage()
		os.Exit(1)
	}

	url := flag.Arg(0)

	if *minbatch > *maxbatch {
		tmp := *minbatch
		*minbatch = *maxbatch
		*maxbatch = tmp
	} else if *minbatch == *maxbatch {
		*maxbatch += 1
	}

	gen := lpxgen.NewGenerator(*minbatch, *maxbatch, parseDist(*logdist))

	client := &http.Client{}
	for i := 0; i < *count; i++ {
		req := gen.Generate(url)

		if resp, err := client.Do(req); err != nil {
			fmt.Fprintf(os.Stderr, "Error while performing request: %q\n", err)
		} else if resp.Status[0] != '2' {
			fmt.Fprintf(os.Stderr, "Non 2xx response recieved: %s\n", resp.Status)
		}
	}
}
