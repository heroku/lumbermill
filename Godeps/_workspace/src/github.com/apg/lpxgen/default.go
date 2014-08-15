package lpxgen

import (
	"fmt"
	"time"
)

type DefaultLog struct{}

func (d DefaultLog) PrivalVersion() string {
	return "<174>1"
}

func (d DefaultLog) Time() string {
	return time.Now().UTC().Format(TimeFormat)
}

func (d DefaultLog) Hostname() string {
	return "localhost"
}

func (d DefaultLog) Name() string {
	return "default"
}

func (d DefaultLog) Procid() string {
	return "-"
}

func (d DefaultLog) Msgid() string {
	return "-"
}

func (d DefaultLog) Msg() string {
	return "default message"
}

func (d DefaultLog) String() string {
	s := FormatSyslog(d)
	return fmt.Sprintf("%d %s", len(s), s)
}
