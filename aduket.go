package aduket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// CapturedRequest stores a received request and its response.
type CapturedRequest struct {
	*http.Request
	BodyContent  []byte
	StatusCode   int
	ResponseBody []byte
}

// Server is a mock HTTP server.
type Server struct {
	*httptest.Server
	Expectations       []*Expectation
	Requests           []*CapturedRequest
	mu                 sync.Mutex
	Upgrader           websocket.Upgrader
	MaxRequestBodySize int64
	OnRequest          func(*CapturedRequest) // Callback for real-time monitoring
}

// NewServer creates and starts a new mock HTTP server.
func NewServer() *Server {
	s := NewUnstartedServer()
	s.Start()
	return s
}

// NewTLSServer creates and starts a new mock HTTPS server.
func NewTLSServer() *Server {
	s := createServer(true)
	s.StartTLS()
	return s
}

// NewUnstartedServer creates a new mock HTTP server but does not start it.
func NewUnstartedServer() *Server {
	return createServer(false)
}

func createServer(tls bool) *Server {
	s := &Server{
		Expectations:       make([]*Expectation, 0),
		Requests:           make([]*CapturedRequest, 0),
		MaxRequestBodySize: 10 * 1024 * 1024, // Default 10MB
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	// Use the handler helper
	s.Server = httptest.NewUnstartedServer(s.handler())
	if tls {
		s.Server.TLS = nil // It will be initialized by StartTLS
	}

	return s
}

func (s *Server) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Panic recovery
		defer func() {
			if rec := recover(); rec != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "mock server panic: %v", rec)
			}
		}()

		s.mu.Lock()
		defer s.mu.Unlock()

		// Body size limit
		if s.MaxRequestBodySize > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, s.MaxRequestBodySize)
		}

		// Record request body
		var bodyBytes []byte
		var err error
		if r.Body != nil {
			bodyBytes, err = io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				fmt.Fprintf(w, "aduket: request body too large: %v", err)
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Record request
		captured := &CapturedRequest{
			Request:     r,
			BodyContent: bodyBytes,
		}

		for _, exp := range s.Expectations {
			if matchExpectation(exp, r) {
				exp.MatchedTimes++

				exp.mu.Lock()
				delay := exp.DelayTime
				responder := exp.Func
				headers := exp.Header
				statusCode := exp.StatusCode
				body := exp.Body
				exp.mu.Unlock()

				captured.StatusCode = statusCode
				captured.ResponseBody = body

				// Handle delay
				if delay > 0 {
					s.mu.Unlock()
					time.Sleep(delay)
					s.mu.Lock()
				}

				if s.OnRequest != nil {
					s.OnRequest(captured)
				}

				// Handle function responder
				if responder != nil {
					responder(w, r)
					return
				}

				// Handle headers
				for k, vv := range headers {
					for _, v := range vv {
						w.Header().Add(k, v)
					}
				}

				w.WriteHeader(statusCode)
				w.Write(body)
				s.Requests = append(s.Requests, captured)
				return
			}
		}

		// Default response if no expectation matches
		captured.StatusCode = http.StatusNotFound
		captured.ResponseBody = []byte(fmt.Sprintf("aduket: no expectation matched for %s %s", r.Method, r.URL.Path))

		if s.OnRequest != nil {
			s.OnRequest(captured)
		}

		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "%s", captured.ResponseBody)
		s.Requests = append(s.Requests, captured)
	})
}

// Listen starts the server on a specific TCP address.
func (s *Server) Listen(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Server != nil {
		s.Server.Close()
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.Server = httptest.NewUnstartedServer(s.handler())
	s.Server.Listener = l
	s.Server.Start()
	return nil
}

// matchExpectation internally checks if a request matches an expectation.
func matchExpectation(exp *Expectation, r *http.Request) bool {
	exp.mu.Lock()
	defer exp.mu.Unlock()
	if exp.Method != "" && exp.Method != r.Method {
		return false
	}
	if exp.Path != "" && exp.Path != r.URL.Path {
		return false
	}
	if exp.Times > 0 && exp.MatchedTimes >= exp.Times {
		return false
	}

	// Match Query Params
	if len(exp.QueryParams) > 0 {
		query := r.URL.Query()
		for k, v := range exp.QueryParams {
			if query.Get(k) != v {
				return false
			}
		}
	}

	return true
}

// Expect registers a new expectation.
func (s *Server) Expect(method, path string) *Expectation {
	if method == "" {
		panic("aduket: method cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	exp := &Expectation{
		Method: method,
		Path:   path,
		Header: make(http.Header),
	}
	s.Expectations = append(s.Expectations, exp)
	return exp
}

// Verify checks if all registered expectations were met.
func (s *Server) Verify(t *testing.T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, exp := range s.Expectations {
		if exp.MatchedTimes == 0 {
			t.Errorf("expected %s %s to be called, but it was not", exp.Method, exp.Path)
		} else if exp.Times > 0 && exp.MatchedTimes < exp.Times {
			t.Errorf("expected %s %s to be called %d times, but it was called %d times", exp.Method, exp.Path, exp.Times, exp.MatchedTimes)
		}
	}
}

// Reset clears all expectations and recorded requests.
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Expectations = make([]*Expectation, 0)
	s.Requests = make([]*CapturedRequest, 0)
}

// RequestCount returns the total number of requests received.
func (s *Server) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Requests)
}

// GetRequest returns the i-th request received.
func (s *Server) GetRequest(i int) *CapturedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.Requests) {
		return nil
	}
	return s.Requests[i]
}

// AssertCalled checks if an expectation for method and path was matched at least once.
func (s *Server) AssertCalled(t *testing.T, method, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, exp := range s.Expectations {
		if exp.Method == method && exp.Path == path && exp.MatchedTimes > 0 {
			return
		}
	}
	t.Errorf("expected %s %s to be called, but it was not", method, path)
}

// AssertNotCalled checks if an expectation for method and path was never matched.
func (s *Server) AssertNotCalled(t *testing.T, method, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, exp := range s.Expectations {
		if exp.Method == method && exp.Path == path && exp.MatchedTimes > 0 {
			t.Errorf("expected %s %s NOT to be called, but it was matched %d times", method, path, exp.MatchedTimes)
			return
		}
	}
}

// AssertRequestCount checks if the total number of requests matches expected count.
func (s *Server) AssertRequestCount(t *testing.T, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Requests) != count {
		t.Errorf("expected %d requests, got %d", count, len(s.Requests))
	}
}

// AssertRequestBodyJSON checks if the request body matches a JSON object.
func (s *Server) AssertRequestBodyJSON(t *testing.T, i int, expected interface{}) {
	req := s.GetRequest(i)
	if req == nil {
		t.Fatalf("request index %d not found", i)
	}

	var actual interface{}
	if err := json.Unmarshal(req.BodyContent, &actual); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}

	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)

	if !bytes.Equal(expectedJSON, actualJSON) {
		t.Errorf("expected body %s, got %s", string(expectedJSON), string(actualJSON))
	}
}

// Upgrade handles WebSocket upgrades.
func (s *Server) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return s.Upgrader.Upgrade(w, r, nil)
}

// AssertHeader checks if a specific request header matches the expected value.
func (s *Server) AssertHeader(t *testing.T, i int, key, value string) {
	req := s.GetRequest(i)
	if req == nil {
		t.Fatalf("request index %d not found", i)
	}

	actual := req.Header.Get(key)
	if actual != value {
		t.Errorf("expected header %s: %s, got %s", key, value, actual)
	}
}

// AssertQueryParam checks if a specific query parameter matches the expected value.
func (s *Server) AssertQueryParam(t *testing.T, i int, key, value string) {
	req := s.GetRequest(i)
	if req == nil {
		t.Fatalf("request index %d not found", i)
	}

	actual := req.URL.Query().Get(key)
	if actual != value {
		t.Errorf("expected query param %s: %s, got %s", key, value, actual)
	}
}
