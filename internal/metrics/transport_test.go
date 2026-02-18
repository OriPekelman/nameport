package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMetricsTransport_BasicRequest(t *testing.T) {
	// Set up a test server that returns a known body.
	body := "hello, world!"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	c := NewCollector()
	transport := &MetricsTransport{
		Wrapped:     http.DefaultTransport,
		ServiceName: "testsvc",
		Collector:   c,
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(data) != body {
		t.Fatalf("body = %q, want %q", string(data), body)
	}

	sm := c.GetMetrics("testsvc")
	if sm == nil {
		t.Fatal("expected metrics for testsvc")
	}
	if atomic.LoadInt64(&sm.TotalRequests) != 1 {
		t.Fatalf("TotalRequests = %d, want 1", sm.TotalRequests)
	}
	if atomic.LoadInt64(&sm.TotalBytesOut) != int64(len(body)) {
		t.Fatalf("TotalBytesOut = %d, want %d", sm.TotalBytesOut, len(body))
	}
	// Active conns should be back to 0 after body close.
	if atomic.LoadInt64(&sm.ActiveConns) != 0 {
		t.Fatalf("ActiveConns = %d, want 0 after close", sm.ActiveConns)
	}

	sm.mu.Lock()
	if sm.StatusCodes[200] != 1 {
		t.Errorf("StatusCodes[200] = %d, want 1", sm.StatusCodes[200])
	}
	sm.mu.Unlock()
}

func TestMetricsTransport_RequestBody(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		receivedBody = string(data)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	c := NewCollector()
	transport := &MetricsTransport{
		Wrapped:     http.DefaultTransport,
		ServiceName: "postsvc",
		Collector:   c,
	}

	client := &http.Client{Transport: transport}
	reqBody := "request payload"
	resp, err := client.Post(server.URL, "text/plain", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()

	if receivedBody != reqBody {
		t.Fatalf("server received %q, want %q", receivedBody, reqBody)
	}

	sm := c.GetMetrics("postsvc")
	if atomic.LoadInt64(&sm.TotalBytesIn) != int64(len(reqBody)) {
		t.Fatalf("TotalBytesIn = %d, want %d", sm.TotalBytesIn, len(reqBody))
	}

	sm.mu.Lock()
	if sm.StatusCodes[201] != 1 {
		t.Errorf("StatusCodes[201] = %d, want 1", sm.StatusCodes[201])
	}
	sm.mu.Unlock()
}

func TestMetricsTransport_StatusCodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewCollector()
	transport := &MetricsTransport{
		Wrapped:     http.DefaultTransport,
		ServiceName: "errsvc",
		Collector:   c,
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	snap := c.Snapshot("errsvc")
	if snap.StatusCodes[404] != 1 {
		t.Errorf("StatusCodes[404] = %d, want 1", snap.StatusCodes[404])
	}
}

func TestMetricsTransport_ResponseTiming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewCollector()
	transport := &MetricsTransport{
		Wrapped:     http.DefaultTransport,
		ServiceName: "timesvc",
		Collector:   c,
	}

	client := &http.Client{Transport: transport}
	for i := 0; i < 10; i++ {
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	snap := c.Snapshot("timesvc")
	if snap.TotalRequests != 10 {
		t.Fatalf("TotalRequests = %d, want 10", snap.TotalRequests)
	}
	// p50 should be non-negative (timing can be 0ms for fast local requests).
	if snap.P50ResponseMs < 0 {
		t.Errorf("P50ResponseMs = %f, expected >= 0", snap.P50ResponseMs)
	}
}

func TestMetricsTransport_NilWrapped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewCollector()
	transport := &MetricsTransport{
		Wrapped:     nil, // should default to http.DefaultTransport
		ServiceName: "defaultsvc",
		Collector:   c,
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt64(&c.GetMetrics("defaultsvc").TotalRequests) != 1 {
		t.Fatal("expected 1 request with nil Wrapped transport")
	}
}
