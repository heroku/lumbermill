package logma

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/bmizerany/lpx"
	"github.com/heroku/lumbermill/Godeps/_workspace/src/github.com/kr/logfmt"
)

// Syslog represents a standard syslog, RFC5426 message, minus structured data
type Syslog struct {
	Prival   int       `json:"prival"`
	Version  int       `json:"version"`
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
	Name     string    `json:"name"`
	Procid   string    `json:"procid"`
	Msgid    string    `json:"msgid"`
	Message  string    `json:"message"`
}

// LpxToEnvelope Attempts to parse a lpx frame into an Event, or returns a Syslog event by default.
func LpxToEnvelope(r *lpx.Reader, owner string) (*Envelope, error) {
	timeStr := string(r.Header().Time)
	eventTime, err := parseTime(timeStr)
	if err != nil {
		// TODO: Increment a counter for time parsing errors
		return nil, err
	}

	header := r.Header()

	// If the syslog Name Header field contains what looks like a log token,
	// let's assume it's an override of the id and we're getting the data from the magic
	// channel
	if bytes.HasPrefix(header.Name, tokenPrefix) {
		owner = string(header.Name)
	}

	var namedEvent interface{}

	msg := r.Bytes()

	// Top level, is it a router log?
	switch {
	case bytes.Equal(header.Procid, routerSentinel):

		// parse as router message
		switch {
		case bytes.Contains(msg, keyCodeH): // router logs with a H error code in them
			// TODO: Increment a routingError lines parsed counter
			re := &RouterError{}
			if err := logfmt.Unmarshal(msg, re); err != nil {
				return nil, err
			}

			namedEvent = re

			// If the app is blank (not pushed) we don't care
			// do nothing atm, increment a counter
		case bytes.Contains(msg, keyCodeBlank), bytes.Contains(msg, keyDescBlank):
			// TODO: Increment some counter.
			// UGH: This is bound to cause problems.
			return nil, nil

			// likely a standard router log
		default:
			// TODO: Increment a counter for a simple router request
			rm := &RouterRequest{}
			if err := logfmt.Unmarshal(msg, rm); err != nil {
				return nil, err
			}

			namedEvent = rm
		}
	case bytes.HasPrefix(msg, dynoErrorSentinel):
		de := &DynoError{}
		byteCode := msg[len(dynoErrorSentinel) : len(dynoErrorSentinel)+2]
		code, err := strconv.Atoi(string(byteCode))
		if err != nil {
			return nil, err
		}
		de.Code = code

		namedEvent = de

	case bytes.Contains(msg, dynoMemMsgSentinel):
		// TODO: Increment a counter for a dynoMem message.
		dm := &DynoMemory{}
		if err := logfmt.Unmarshal(msg, dm); err != nil {
			return nil, err
		}

		namedEvent = dm

	case bytes.Contains(msg, dynoLoadMsgSentinel):
		// TODO: Increment a counter for a dynoLoad message.
		dl := &DynoLoad{}
		if err := logfmt.Unmarshal(msg, dl); err != nil {
			return nil, err
		}

		namedEvent = dl

	default: // just make it a syslog message and be done.
		// TODO: Increment a counter for a general syslog message.

		prival, _ := strconv.Atoi(string(header.PrivalVersion[1:4]))
		version, _ := strconv.Atoi(string(header.PrivalVersion[5:]))
		namedEvent = &Syslog{
			Prival:   prival,
			Version:  version,
			Time:     eventTime,
			Hostname: string(header.Hostname),
			Name:     string(header.Name),
			Procid:   string(header.Procid),
			Msgid:    string(header.Msgid),
			Message:  string(msg),
		}
	}

	return &Envelope{
		Type:  typename(namedEvent),
		Time:  eventTime.Unix(),
		Owner: owner,
		Value: namedEvent,
	}, nil
}

func parseTime(timeStr string) (time.Time, error) {
	var empty time.Time
	t, e := time.Parse("2006-01-02T15:04:05.000000+00:00", timeStr)
	if e != nil {
		t, e = time.Parse("2006-01-02T15:04:05+00:00", timeStr)
		if e != nil {
			return empty, fmt.Errorf("Error Parsing Time(%s): %q\n", timeStr, e)
		}
	}
	return t, e
}
