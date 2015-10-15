package logma

import (
	"bytes"
	"strconv"
	"strings"
)

// DynoError represents a dyno error
type DynoError struct {
	Code int `json:"code"`
}

// DynoLoad represents a load measurement taken from a dyno
type DynoLoad struct {
	Source       string  `json:"source"`
	Dyno         string  `json:"dyno"`
	LoadAvg1Min  float64 `json:"load_avg_1m"`
	LoadAvg5Min  float64 `json:"load_avg_5m"`
	LoadAvg15Min float64 `json:"load_avg_15m"`
}

// HandleLogFmt converts a field from logfmt to a DynoLoad message.
func (d *DynoLoad) HandleLogfmt(key, val []byte) error {
	switch {
	case bytes.Equal(key, keySource):
		d.Source = string(val)
	case bytes.Equal(key, keyDyno):
		d.Dyno = string(val)
	case bytes.HasSuffix(key, keyLoadAvg1Min):
		d.LoadAvg1Min, _ = strconv.ParseFloat(string(val), 64)
	case bytes.HasSuffix(key, keyLoadAvg5Min):
		d.LoadAvg5Min, _ = strconv.ParseFloat(string(val), 64)
	case bytes.HasSuffix(key, keyLoadAvg15Min):
		d.LoadAvg15Min, _ = strconv.ParseFloat(string(val), 64)
	}
	return nil
}

// DynoMemory represents a memory measurement taken from a dyno
type DynoMemory struct {
	Source        string  `json:"source"`
	Dyno          string  `json:"dyno"`
	MemoryTotal   float64 `json:"memory_total"`
	MemoryRSS     float64 `json:"memory_rss"`
	MemoryCache   float64 `json:"memory_cache"`
	MemorySwap    float64 `json:"memory_swap"`
	MemoryPgpgin  int     `json:"memory_pgpgin"`
	MemoryPgpgout int     `json:"memory_pgpgout"`
}

// HandleLogfmt implements a logfmt unmarshaller
func (d *DynoMemory) HandleLogfmt(key, val []byte) error {
	switch {
	case bytes.Equal(key, keySource):
		d.Source = string(val)
	case bytes.Equal(key, keyDyno):
		d.Dyno = string(val)
	case bytes.HasSuffix(key, keyMemoryTotal):
		d.MemoryTotal, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, keyMemoryRSS):
		d.MemoryRSS, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, keyMemoryCache):
		d.MemoryCache, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, keyMemorySwap):
		d.MemorySwap, _ = strconv.ParseFloat(strings.TrimSuffix(string(val), "MB"), 64)
	case bytes.HasSuffix(key, keyMemoryPgpgin):
		d.MemoryPgpgin, _ = strconv.Atoi(strings.TrimSuffix(string(val), "pages"))
	case bytes.HasSuffix(key, keyMemoryPgpgout):
		d.MemoryPgpgout, _ = strconv.Atoi(strings.TrimSuffix(string(val), "pages"))
	}
	return nil
}
