package authenticater

import (
	"net/http"
	"strings"
	"sync"
)

// LogplexDrainToken ensures that the Logplex-Drain-Token header contains a
// known token and is safe for concurrent use
type LogplexDrainToken struct {
	sync.RWMutex
	tokens map[string]struct{}
}

// NewLogplexDrainToken creates and returns a new LogplexDrainTokens
func NewLogplexDrainToken() *LogplexDrainToken {
	return &LogplexDrainToken{tokens: make(map[string]struct{})}
}

// NewLogplexDrainTokenFromString creates and returns a new LogplexDrainTokens
// instance from the provided string containing tokens in the folloing format:
// token1,token2,token3,...
func NewLogplexDrainTokenFromString(tokens string) *LogplexDrainToken {
	ldt := NewLogplexDrainToken()
	for _, token := range strings.Split(tokens, ",") {
		ldt.AddToken(token)
	}
	return ldt
}

// AddToken adds a token to the list of acceptable tokens.
func (ldt *LogplexDrainToken) AddToken(token string) {
	ldt.Lock()
	ldt.tokens[token] = struct{}{}
	ldt.Unlock()
}

// Authenticate the request if the Logplex-Drain-Token header contains a known
// token
func (ldt *LogplexDrainToken) Authenticate(r *http.Request) (exists bool) {
	if token := r.Header.Get("Logplex-Drain-Token"); token != "" {
		ldt.RLock()
		_, exists = ldt.tokens[token]
		ldt.RUnlock()
	}
	return
}
