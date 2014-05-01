package main

import (
	"bytes"
	"strconv"
	"strings"
)

type dynoMsg struct {
	Source        string
	Dyno          string
	MemoryTotal   float64
	MemoryRSS     float64
	MemoryCache   float64
	MemorySwap    float64
	MemoryPgpgin  int
	MemoryPgpgout int
}

var (
	source        = []byte("source")
	dyno          = []byte("dyno")
	memoryTotal   = []byte("memory_total")
	memoryRSS     = []byte("memory_rss")
	memoryCache   = []byte("memory_cache")
	memorySwap    = []byte("memory_swap")
	memoryPgpgin  = []byte("memory_pgpgin")
	memoryPgpgout = []byte("memory_pgpgout")
)

func (dm *dynoMsg) HandleLogfmt(key, val []byte) error {
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
