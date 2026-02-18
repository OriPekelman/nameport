package metrics

import (
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// MetricsTransport wraps an http.RoundTripper and records request/response
// metrics (timing, bytes, status codes) into a Collector.
type MetricsTransport struct {
	// Wrapped is the underlying RoundTripper. If nil, http.DefaultTransport is used.
	Wrapped http.RoundTripper

	// ServiceName identifies the service these metrics belong to.
	ServiceName string

	// Collector receives the recorded metrics.
	Collector *Collector
}

// RoundTrip implements http.RoundTripper. It measures timing, counts bytes,
// and records the status code of each request/response pair.
func (t *MetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.Wrapped
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Count request body bytes.
	var bytesIn int64
	if req.Body != nil {
		req.Body = &countingReadCloser{ReadCloser: req.Body, n: &bytesIn}
	}

	t.Collector.IncrementActiveConns(t.ServiceName)

	start := time.Now()
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Collector.DecrementActiveConns(t.ServiceName)
		return nil, err
	}

	// Wrap the response body to count bytes read and record metrics on close.
	var bytesOut int64
	resp.Body = &metricsBody{
		ReadCloser:  resp.Body,
		bytesOut:    &bytesOut,
		onClose: func() {
			duration := time.Since(start)
			t.Collector.DecrementActiveConns(t.ServiceName)
			t.Collector.RecordRequest(t.ServiceName, resp.StatusCode, bytesIn, atomic.LoadInt64(&bytesOut), duration)
		},
	}
	return resp, nil
}

// countingReadCloser wraps an io.ReadCloser and counts bytes read.
type countingReadCloser struct {
	io.ReadCloser
	n *int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	atomic.AddInt64(c.n, int64(n))
	return n, err
}

// metricsBody wraps the response body to count bytes and fire a callback on Close.
type metricsBody struct {
	io.ReadCloser
	bytesOut *int64
	onClose  func()
	closed   bool
}

func (mb *metricsBody) Read(p []byte) (int, error) {
	n, err := mb.ReadCloser.Read(p)
	atomic.AddInt64(mb.bytesOut, int64(n))
	return n, err
}

func (mb *metricsBody) Close() error {
	err := mb.ReadCloser.Close()
	if !mb.closed {
		mb.closed = true
		mb.onClose()
	}
	return err
}
