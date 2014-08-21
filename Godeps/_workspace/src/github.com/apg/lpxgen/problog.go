package lpxgen

import (
	"fmt"
	"math/rand"
	"sync"
)

type ProbLog struct {
	logs       map[Log]float32
	sum        float32
	mutex      sync.Mutex
	currentLog Log
}

func NewProbLog(logs map[Log]float32) *ProbLog {
	var sum float32
	for _, v := range logs {
		sum += v
	}

	return &ProbLog{
	  logs: logs,
		currentLog: DefaultLog{},
	}
}

func (p *ProbLog) Add(l Log, prob float32) {
	defer p.mutex.Unlock()
	p.mutex.Lock()

	p.logs[l] = prob
	p.sum += prob
}

func (p *ProbLog) PrivalVersion() string {
	return p.currentLog.PrivalVersion()
}

func (p *ProbLog) Time() string {
	return p.currentLog.Time()
}

func (p *ProbLog) Hostname() string {
	return p.currentLog.Hostname()
}

func (p *ProbLog) Name() string {
	return p.currentLog.Name()
}

func (p *ProbLog) Procid() string {
	return p.currentLog.Procid()
}

func (p *ProbLog) Msgid() string {
	return p.currentLog.Msgid()
}

func (p *ProbLog) Msg() string {
	return p.currentLog.Msg()
}

func (p *ProbLog) String() string {
	defer p.mutex.Unlock()

	p.mutex.Lock()
	p.currentLog = p.nextLog()
  s := FormatSyslog(p.currentLog)
	fmt.Println(s)
	return fmt.Sprintf("%d %s", len(s), s)
}

func (p *ProbLog) nextLog() Log {
	var l Log
	var v float32

	if len(p.logs) == 0 {
		return p.currentLog
	}

	value := rand.Float32() * p.sum

	for l, v = range p.logs {
		value -= v
		if v < 0 {
			return l
		}
	}

	return l
}
