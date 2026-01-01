package aduket

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestServer(t *testing.T) {
	s := NewServer()
	defer s.Close()

	s.Expect("GET", "/hello").Response(http.StatusOK, "world")

	resp, err := http.Get(s.URL + "/hello")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	if string(body) != "world" {
		t.Errorf("expected body 'world', got '%s'", string(body))
	}

	if s.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", s.RequestCount())
	}
}

func TestDelay(t *testing.T) {
	s := NewServer()
	defer s.Close()

	s.Expect("GET", "/slow").Delay(100*time.Millisecond).Response(http.StatusOK, "slow")

	start := time.Now()
	http.Get(s.URL + "/slow")
	elapsed := time.Since(start)

	if elapsed < 100*time.Millisecond {
		t.Errorf("expected at least 100ms delay, got %v", elapsed)
	}
}

func TestDynamicResponder(t *testing.T) {
	s := NewServer()
	defer s.Close()

	s.Expect("POST", "/echo").RespondWith(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("dynamic"))
	})

	resp, _ := http.Post(s.URL+"/echo", "text/plain", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", resp.StatusCode)
	}
	if string(body) != "dynamic" {
		t.Errorf("expected body 'dynamic', got '%s'", string(body))
	}
}

func TestQueryParams(t *testing.T) {
	s := NewServer()
	defer s.Close()

	s.Expect("GET", "/search").WithQuery("q", "aduket").Response(http.StatusOK, "found")

	// Missing query param
	resp1, _ := http.Get(s.URL + "/search")
	if resp1.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for missing query param, got %d", resp1.StatusCode)
	}

	// Correct query param
	resp2, _ := http.Get(s.URL + "/search?q=aduket")
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for correct query param, got %d", resp2.StatusCode)
	}
}

func TestTLSServer(t *testing.T) {
	s := NewTLSServer()
	defer s.Close()

	s.Expect("GET", "/secure").Response(http.StatusOK, "secure")

	// Need to disable cert verification for httptest TLS server
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(s.URL + "/secure")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestJSONAssertion(t *testing.T) {
	s := NewServer()
	defer s.Close()

	s.Expect("POST", "/json").Response(http.StatusOK, "ok")

	payload := map[string]string{"name": "ismail"}
	jsonBytes, _ := json.Marshal(payload)
	http.Post(s.URL+"/json", "application/json", bytes.NewBuffer(jsonBytes))

	s.AssertRequestBodyJSON(t, 0, payload)
}

func TestWebSocket(t *testing.T) {
	s := NewServer()
	defer s.Close()

	s.Expect("GET", "/ws").RespondWith(func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.Upgrade(w, r)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			err = conn.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	})

	wsURL := strings.Replace(s.URL, "http", "ws", 1) + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello")
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	_, received, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	if string(received) != string(msg) {
		t.Errorf("expected message '%s', got '%s'", string(msg), string(received))
	}
}
