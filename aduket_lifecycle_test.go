package aduket

import (
	"net/http"
	"testing"
)

func TestUnstartedServer(t *testing.T) {
	s := NewUnstartedServer()
	if s.URL != "" {
		t.Errorf("expected empty URL for unstarted server, got %s", s.URL)
	}
	s.Start()
	defer s.Close()
	if s.URL == "" {
		t.Error("expected URL after starting server")
	}
}

func TestListen(t *testing.T) {
	s := NewServer()
	defer s.Close()

	// Pick a random free port for testing Listen
	if err := s.Listen("127.0.0.1:0"); err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}

	s.Expect("GET", "/test").Response(200, "ok")

	resp, err := http.Get(s.URL + "/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
