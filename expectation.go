package aduket

import (
	"net/http"
	"sync"
	"time"
)

// Responder is a function that generates a response based on the request.
type Responder func(w http.ResponseWriter, r *http.Request)

// Expectation represents a mocked request and its response.
type Expectation struct {
	Method       string
	Path         string
	StatusCode   int
	Body         []byte
	Header       http.Header
	Times        int // Number of times this expectation can be matched, 0 means unlimited
	MatchedTimes int
	DelayTime    time.Duration
	Func         Responder
	QueryParams  map[string]string
	mu           sync.Mutex
}

// Response sets the response status and body for the expectation.
func (e *Expectation) Response(status int, body string) *Expectation {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.StatusCode = status
	e.Body = []byte(body)
	return e
}

// Headers sets the response headers for the expectation.
func (e *Expectation) Headers(headers map[string]string) *Expectation {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range headers {
		e.Header.Set(k, v)
	}
	return e
}

// TimesSet sets how many times this expectation should match.
func (e *Expectation) TimesSet(n int) *Expectation {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Times = n
	return e
}

// Delay sets a simulated delay before responding.
func (e *Expectation) Delay(d time.Duration) *Expectation {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.DelayTime = d
	return e
}

// RespondWith sets a dynamic responder function.
func (e *Expectation) RespondWith(f Responder) *Expectation {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Func = f
	return e
}

// WithQuery adds a query parameter requirement to the expectation.
func (e *Expectation) WithQuery(key, value string) *Expectation {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.QueryParams == nil {
		e.QueryParams = make(map[string]string)
	}
	e.QueryParams[key] = value
	return e
}
