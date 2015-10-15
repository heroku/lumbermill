package logma

import (
	"bytes"
	"strconv"
	"strings"
)

// RouterRequest represents an actual request to the Heroku platform.
type RouterRequest struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Host      string `json:"host"`
	RequestID string `json:"request_id"`
	Fwd       string `json:"fwd"`
	Dyno      string `json:"dyno"`
	Connect   int    `json:"connect"`
	Service   int    `json:"service"`
	Status    int    `json:"status"`
	Bytes     int    `json:"bytes"`
}

// HandleLogfmt implements the logfmt unmarshaller
func (r *RouterRequest) HandleLogfmt(key, val []byte) error {
	switch {
	case bytes.Equal(key, keyMethod):
		r.Method = string(val)
	case bytes.Equal(key, keyPath):
		r.Path = string(val)
	case bytes.Equal(key, keyHost):
		r.Host = string(val)
	case bytes.Equal(key, keyRequestID):
		r.RequestID = string(val)
	case bytes.Equal(key, keyFwd):
		r.Fwd = string(val)
	case bytes.Equal(key, keyDyno):
		r.Dyno = string(val)
	case bytes.Equal(key, keyConnect):
		connect, e := strconv.Atoi(strings.TrimSuffix(string(val), "ms"))
		if e != nil {
			return e
		}
		r.Connect = connect
	case bytes.Equal(key, keyService):
		service, e := strconv.Atoi(strings.TrimSuffix(string(val), "ms"))
		if e != nil {
			return e
		}
		r.Service = service
	case bytes.Equal(key, keyStatus):
		status, e := strconv.Atoi(string(val))
		if e != nil {
			return e
		}
		r.Status = status
	case bytes.Equal(key, keyBytes):
		bytes, e := strconv.Atoi(string(val))
		if e != nil {
			return e
		}
		r.Bytes = bytes
	default:
		return nil
		// log.Printf("Unknown key (%s) with value: %s\n", key, string(val))
	}
	return nil
}

// RouterError represents an error generated by the Heroku router.
type RouterError struct {
	At        string `json:"at"`
	Code      string `json:"code"`
	Desc      string `json:"desc"`
	Method    string `json:"method"`
	Host      string `json:"host"`
	Fwd       string `json:"fwd"`
	Dyno      string `json:"dyno"`
	Path      string `json:"path"`
	RequestID string `json:"request_id"`
	Connect   int    `json:"connect"`
	Service   int    `json:"service"`
	Status    int    `json:"status"`
	Bytes     int    `json:"bytes"`
	Sock      string `json:"sock"`
}

// Typename returns the name of this event type, and implmeents Typenamer
func (r *RouterError) Typename() string {
	return "routererror"
}

func (r *RouterError) HandleLogfmt(key, val []byte) error {
	switch {
	case bytes.Equal(key, keyAt):
		r.At = string(val)
	case bytes.Equal(key, keyCode):
		r.Code = string(val)
	case bytes.Equal(key, keyDesc):
		r.Desc = string(val)
	case bytes.Equal(key, keyMethod):
		r.Method = string(val)
	case bytes.Equal(key, keyHost):
		r.Host = string(val)
	case bytes.Equal(key, keyFwd):
		r.Fwd = string(val)
	case bytes.Equal(key, keyPath):
		r.Path = string(val)
	case bytes.Equal(key, keyRequestID):
		r.RequestID = string(val)
	case bytes.Equal(key, keyDyno):
		r.Dyno = string(val)
	case bytes.Equal(key, keyConnect):
		connect, _ := strconv.Atoi(strings.TrimSuffix(string(val), "ms"))
		// swallow errors because connect could be nothing
		r.Connect = connect
	case bytes.Equal(key, keyService):
		service, _ := strconv.Atoi(strings.TrimSuffix(string(val), "ms"))
		// swallow errors because service could be nothing
		r.Service = service
	case bytes.Equal(key, keyStatus):
		status, _ := strconv.Atoi(string(val))
		// swallow errors because status could be nothing
		r.Status = status
	case bytes.Equal(key, keyBytes):
		bytes, _ := strconv.Atoi(string(val))
		// swallow errors because bytes could be nothing
		r.Bytes = bytes
	case bytes.Equal(key, keySock):
		r.Sock = string(val)
	default:
		return nil
		// log.Printf("Unknown key (%s) with value: %s\n", key, string(val))
	}
	return nil
}