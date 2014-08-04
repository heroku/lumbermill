package main

import (
	"bytes"
	"strconv"
	"strings"
)

var (
	keyAt        = []byte("at")
	keyCode      = []byte("code")
	keyDesc      = []byte("desc")
	keyMethod    = []byte("method")
	keyPath      = []byte("path")
	keyHost      = []byte("host")
	keyRequestId = []byte("request_id")
	keyFwd       = []byte("fwd")
	keyConnect   = []byte("connect")
	keyService   = []byte("service")
	keyStatus    = []byte("status")
	keySock      = []byte("sock")
	keyBytes     = []byte("bytes")
	keyCodeH     = []byte("code=H")
	keyCodeBlank = []byte("code=blank-app")
	keyDescBlank = []byte("desc=\"Blank app\"")
)

// at=info method=GET path=/check?metric=railgun.accepting:sum:max,railgun.running:sum:max&0
// host=umpire.herokai.com request_id=1f3ed8a9-c80c-49de-a4af-2df9f4ddb858 fwd="46.20.45.18"
// dyno=web.14 connect=1ms service=849ms status=500 bytes=306
type routerMsg struct {
	Method    string
	Path      string
	Host      string
	RequestId string
	Fwd       string
	Dyno      string
	Connect   int
	Service   int
	Status    int
	Bytes     int
}

func (rm *routerMsg) HandleLogfmt(key, val []byte) error {
	switch {
	case bytes.Equal(key, keyMethod):
		rm.Method = string(val)
	case bytes.Equal(key, keyPath):
		rm.Path = string(val)
	case bytes.Equal(key, keyHost):
		rm.Host = string(val)
	case bytes.Equal(key, keyRequestId):
		rm.RequestId = string(val)
	case bytes.Equal(key, keyFwd):
		rm.Fwd = string(val)
	case bytes.Equal(key, keyDyno):
		rm.Dyno = string(val)
	case bytes.Equal(key, keyConnect):
		connect, e := strconv.Atoi(strings.TrimSuffix(string(val), "ms"))
		if e != nil {
			return e
		}
		rm.Connect = connect
	case bytes.Equal(key, keyService):
		service, e := strconv.Atoi(strings.TrimSuffix(string(val), "ms"))
		if e != nil {
			return e
		}
		rm.Service = service
	case bytes.Equal(key, keyStatus):
		status, e := strconv.Atoi(string(val))
		if e != nil {
			return e
		}
		rm.Status = status
	case bytes.Equal(key, keyBytes):
		bytes, e := strconv.Atoi(string(val))
		if e != nil {
			return e
		}
		rm.Bytes = bytes
	default:
		return nil
		// log.Printf("Unknown key (%s) with value: %s\n", key, string(val))
	}
	return nil
}

type routerError struct {
	At        string
	Code      string
	Desc      string
	Method    string
	Host      string
	Fwd       string
	Dyno      string
	Path      string
	RequestId string
	Connect   int
	Service   int
	Status    int
	Bytes     int
	Sock      string
}

func (re *routerError) HandleLogfmt(key, val []byte) error {
	switch {
	case bytes.Equal(key, keyAt):
		re.At = string(val)
	case bytes.Equal(key, keyCode):
		re.Code = string(val)
	case bytes.Equal(key, keyDesc):
		re.Desc = string(val)
	case bytes.Equal(key, keyMethod):
		re.Method = string(val)
	case bytes.Equal(key, keyHost):
		re.Host = string(val)
	case bytes.Equal(key, keyFwd):
		re.Fwd = string(val)
	case bytes.Equal(key, keyPath):
		re.Path = string(val)
	case bytes.Equal(key, keyRequestId):
		re.RequestId = string(val)
	case bytes.Equal(key, keyDyno):
		re.Dyno = string(val)
	case bytes.Equal(key, keyConnect):
		connect, _ := strconv.Atoi(strings.TrimSuffix(string(val), "ms"))
		// swallow errors because connect could be nothing
		re.Connect = connect
	case bytes.Equal(key, keyService):
		service, _ := strconv.Atoi(strings.TrimSuffix(string(val), "ms"))
		// swallow errors because service could be nothing
		re.Service = service
	case bytes.Equal(key, keyStatus):
		status, _ := strconv.Atoi(string(val))
		// swallow errors because status could be nothing
		re.Status = status
	case bytes.Equal(key, keyBytes):
		bytes, _ := strconv.Atoi(string(val))
		// swallow errors because bytes could be nothing
		re.Bytes = bytes
	case bytes.Equal(key, keySock):
		re.Sock = string(val)
	default:
		return nil
		// log.Printf("Unknown key (%s) with value: %s\n", key, string(val))
	}
	return nil
}
