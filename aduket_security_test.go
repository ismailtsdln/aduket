package aduket

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestVerify(t *testing.T) {
	s := NewServer()
	defer s.Close()

	s.Expect("GET", "/ping").Response(http.StatusOK, "pong")

	// Verify should fail if not called
	mockT := &testing.T{}
	s.Verify(mockT)
	if !mockT.Failed() {
		t.Errorf("expected Verify to fail for unmatched expectation")
	}

	http.Get(s.URL + "/ping")
	s.Verify(t) // Should pass now
}

func TestPanicRecovery(t *testing.T) {
	s := NewServer()
	defer s.Close()

	s.Expect("GET", "/panic").RespondWith(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	resp, err := http.Get(s.URL + "/panic")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	if !strings.Contains(string(body), "mock server panic") {
		t.Errorf("expected error message to contain 'mock server panic', got '%s'", string(body))
	}
}

func TestBodySizeLimit(t *testing.T) {
	s := NewServer()
	s.MaxRequestBodySize = 100 // 100 bytes limit
	defer s.Close()

	s.Expect("POST", "/large").Response(http.StatusOK, "ok")

	largeBody := strings.Repeat("a", 200)
	resp, _ := http.Post(s.URL+"/large", "text/plain", strings.NewReader(largeBody))

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("expected error 413 for exceeding body size, got %d", resp.StatusCode)
	}
}

func TestMethodValidation(t *testing.T) {
	s := NewServer()
	defer s.Close()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for empty method")
		}
	}()

	s.Expect("", "/path")
}

// ... rest of the existing tests ...
