package lpxgen

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
)

const (
	TimeFormat = "2006-01-02T15:04:05.000000+00:00"
)

type Log interface {
	PrivalVersion() string
	Time() string
	Hostname() string
	Name() string
	Procid() string
	Msgid() string
	Msg() string
	String() string
}

type LPXGenerator struct {
	Mincount int
	Maxcount int
	Log      Log
}

func NewGenerator(minBatch, maxBatch int, loggen Log) *LPXGenerator {
	return &LPXGenerator{
		Mincount: minBatch,
		Maxcount: maxBatch,
		Log:      loggen,
	}
}

func (g *LPXGenerator) Generate(url string) *http.Request {
	var body bytes.Buffer
	batchSize := g.Mincount + rand.Intn(g.Maxcount-g.Mincount)

	for i := 0; i < batchSize; i++ {
		body.WriteString(g.Log.String())
	}

	request, _ := http.NewRequest("POST", url, bytes.NewReader(body.Bytes()))
	request.Header.Add("Content-Length", string(body.Len()))
	request.Header.Add("Content-Type", "application/logplex-1")
	request.Header.Add("Logplex-Msg-Count", string(batchSize))
	request.Header.Add("Logplex-Frame-Id", RandomFrameId())
	request.Header.Add("Logplex-Drain-Token", RandomDrainToken())
	return request
}

func FormatSyslog(l Log) string {
	return fmt.Sprintf("%s %s %s %s %s %s %s\n",
		l.PrivalVersion(),
		l.Time(),
		l.Hostname(),
		l.Name(),
		l.Procid(),
		l.Msgid(),
		l.Msg())
}

var hexchars = []byte("0123456789abcdef")

func randomHexString(i int) string {
	x := make([]byte, i)
	for i > 0 {
		i--
		x[i] = hexchars[rand.Intn(16)]
	}
	return string(x)
}

func UUID4() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		randomHexString(8),
		randomHexString(4),
		randomHexString(4),
		randomHexString(4),
		randomHexString(12))
}

func RandomFrameId() string {
	return randomHexString(32)
}

func RandomDrainToken() string {
	return fmt.Sprintf("t.%s", UUID4())
}

func RandomIPv4() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rand.Intn(255),
		rand.Intn(255),
		rand.Intn(255),
		rand.Intn(255))
}
