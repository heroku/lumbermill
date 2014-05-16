package main

import (
	"bytes"
	"strconv"
	"strings"
)

var (
	keySource           = []byte("source")
	keyDyno             = []byte("dyno")
	keyMemoryTotal      = []byte("memory_total")
	keyMemoryRSS        = []byte("memory_rss")
	keyMemoryCache      = []byte("memory_cache")
	keyMemorySwap       = []byte("memory_swap")
	keyMemoryPgpgin     = []byte("memory_pgpgin")
	keyMemoryPgpgout    = []byte("memory_pgpgout")
	keyLoadAvg1Min      = []byte("load_avg_1m")
	keyLoadAvg5Min      = []byte("load_avg_5m")
	keyLoadAvg15Min     = []byte("load_avg_15m")
	dynoMemMsgSentinel  = []byte("sample#memory_total")
	dynoLoadMsgSentinel = []byte("sample#load_avg_1m")
	dynoErrorSentinel   = []byte("Error R")
)

type dynoError struct {
	Code int
}

func parseBytesToDynoError(msg []byte) (dynoError, error) {
	de := dynoError{}
	byteCode := msg[len(dynoErrorSentinel) : len(dynoErrorSentinel)+2]
	code, err := strconv.Atoi(string(byteCode))
	if err != nil {
		return de, err
	}
	de.Code = code
	return de, nil
}

type dynoMemMsg struct {
	Source        string
	Dyno          string
	MemoryTotal   float64
	MemoryRSS     float64
	MemoryCache   float64
	MemorySwap    float64
	MemoryPgpgin  int
	MemoryPgpgout int
}

func (dm *dynoMemMsg) HandleLogfmt(key, val []byte) error {
	switch {
	case bytes.Equal(key, keySource):
		dm.Source = string(val)
	case bytes.Equal(key, keyDyno):
		dm.Dyno = string(val)
	case bytes.HasSuffix(key, keyMemoryTotal):
		dm.MemoryTotal, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, keyMemoryRSS):
		dm.MemoryRSS, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, keyMemoryCache):
		dm.MemoryCache, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, keyMemorySwap):
		dm.MemorySwap, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, keyMemoryPgpgin):
		dm.MemoryPgpgin, _ = strconv.Atoi(strings.TrimSuffix(string(val), "pages"))
	case bytes.HasSuffix(key, keyMemoryPgpgout):
		dm.MemoryPgpgout, _ = strconv.Atoi(strings.TrimSuffix(string(val), "pages"))
	}
	return nil
}

type dynoLoadMsg struct {
	Source       string
	Dyno         string
	LoadAvg1Min  float64
	LoadAvg5Min  float64
	LoadAvg15Min float64
}

func (dm *dynoLoadMsg) HandleLogfmt(key, val []byte) error {
	switch {
	case bytes.Equal(key, keySource):
		dm.Source = string(val)
	case bytes.Equal(key, keyDyno):
		dm.Dyno = string(val)
	case bytes.HasSuffix(key, keyLoadAvg1Min):
		dm.LoadAvg1Min, _ = strconv.ParseFloat(string(val), 64)
	case bytes.HasSuffix(key, keyLoadAvg5Min):
		dm.LoadAvg5Min, _ = strconv.ParseFloat(string(val), 64)
	case bytes.HasSuffix(key, keyLoadAvg15Min):
		dm.LoadAvg15Min, _ = strconv.ParseFloat(string(val), 64)
	}
	return nil
}
