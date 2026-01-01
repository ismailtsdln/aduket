# Aduket

Aduket is a straight-forward HTTP client testing tool for Go. It provides a lean way to spin up a mock HTTP server to imitate different responses and assert that your HTTP client is behaving as expected.

## Features

- **Lean Mock Server**: Easily spin up HTTP or HTTPS (TLS) servers.
- **Expectations**: Register expectations for specific methods, paths, and query parameters.
- **Simulated Delays**: Test how your client handles slow responses.
- **Dynamic Responders**: Use full `http.HandlerFunc` logic for complex responses.
- **WebSocket Support**: Easily test WebSocket connections and message flows.
- **Request Recording**: Every request is recorded and can be retrieved for detailed inspection.
- **Assertion Helpers**: Built-in helpers for status codes, call counts, and JSON bodies.

## Installation

```bash
go get github.com/ismailtsdln/aduket
```

## Usage

### Simple Mocking

```go
s := aduket.NewServer()
defer s.Close()

s.Expect("GET", "/hello").Response(http.StatusOK, "world")

resp, _ := http.Get(s.URL + "/hello")
// ...
s.AssertCalled(t, "GET", "/hello")
```

### Simulated Delays

```go
s.Expect("GET", "/slow").
    Delay(2 * time.Second).
    Response(http.StatusOK, "slow response")
```

### Dynamic Responders & WebSockets

```go
s.Expect("GET", "/ws").RespondWith(func(w http.ResponseWriter, r *http.Request) {
    conn, _ := s.Upgrade(w, r)
    defer conn.Close()
    // Handle websocket...
})
```

### JSON Body Assertions

```go
s.Expect("POST", "/api/user").Response(http.StatusCreated, "Created")

// After making a request with JSON body:
s.AssertRequestBodyJSON(t, 0, map[string]interface{}{
    "name": "ismail",
})
```

### Query Parameter Matching

```go
s.Expect("GET", "/search").
    WithQuery("q", "aduket").
    Response(http.StatusOK, "found")
```

### HTTPS/TLS Support

```go
s := aduket.NewTLSServer()
defer s.Close()
// Use s.URL with a client configured to Trust the server (or InsecureSkipVerify)
```

## CLI Interface

Aduket comes with a visually rich TUI for real-time monitoring of your mock server.

### Starting the CLI

```bash
go run cmd/aduket/main.go
```

### Configuration (Optional)

You can load expectations from a JSON file:

```json
{
  "expectations": [
    {
      "method": "GET",
      "path": "/api/v1/health",
      "status": 200,
      "response": "{\"status\":\"ok\"}"
    }
  ]
}
```

Run with:

```bash
go run cmd/aduket/main.go -config config.json
```

### TUI Features

- **Real-time Monitoring**: See requests as they hit the server.
- **Request Inspection**: Select a request to see full headers and body.
- **Side-by-side Layout**: Modern dashboard with filter/search capabilities.
- **Visual Feedback**: Color-coded HTTP methods and premium styling.

- **Panic Recovery**: The mock server automatically recovers from panics in your responders and returns a 500 status.
- **Request Size Limiting**: Control memory usage with `s.MaxRequestBodySize`.
- **Automatic Verification**: Use `s.Verify(t)` at the end of your test to ensure all registered expectations were met.
- **Improved Errors**: Clear error messages when no expectation matches provide details about the received method and path.
- **Validation**: Empty method expectations will trigger a panic to catch configuration errors early.

## License

MIT
