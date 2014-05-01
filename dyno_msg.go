package main

import (
	"bytes"
	"strconv"
	"strings"
)

var (
	source        = []byte("source")
	dyno          = []byte("dyno")
	memoryTotal   = []byte("memory_total")
	memoryRSS     = []byte("memory_rss")
	memoryCache   = []byte("memory_cache")
	memorySwap    = []byte("memory_swap")
	memoryPgpgin  = []byte("memory_pgpgin")
	memoryPgpgout = []byte("memory_pgpgout")
	loadAvg1Min   = []byte("load_avg_1m")
	loadAvg5Min   = []byte("load_avg_5m")
	loadAvg15Min  = []byte("load_avg_15m")
)

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
	case bytes.Equal(key, source):
		dm.Source = string(val)
	case bytes.Equal(key, dyno):
		dm.Dyno = string(val)
	case bytes.HasSuffix(key, memoryTotal):
		dm.MemoryTotal, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, memoryRSS):
		dm.MemoryRSS, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, memoryCache):
		dm.MemoryCache, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, memorySwap):
		dm.MemorySwap, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, memoryPgpgin):
		dm.MemoryPgpgin, _ = strconv.Atoi(strings.TrimSuffix(string(val), "pages"))
	case bytes.HasSuffix(key, memoryPgpgout):
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
	case bytes.Equal(key, source):
		dm.Source = string(val)
	case bytes.Equal(key, dyno):
		dm.Dyno = string(val)
	case bytes.HasSuffix(key, loadAvg1Min):
		dm.LoadAvg1Min, _ = strconv.ParseFloat(string(val), 64)
	case bytes.HasSuffix(key, loadAvg5Min):
		dm.LoadAvg5Min, _ = strconv.ParseFloat(string(val), 64)
	case bytes.HasSuffix(key, loadAvg15Min):
		dm.LoadAvg15Min, _ = strconv.ParseFloat(string(val), 64)
	}
	return nil
}
