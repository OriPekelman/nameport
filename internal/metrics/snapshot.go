package metrics

import "sync/atomic"

// MetricsSnapshot is a point-in-time, JSON-serializable view of a service's metrics.
type MetricsSnapshot struct {
	ServiceName    string       `json:"service_name"`
	ActiveConns    int64        `json:"active_conns"`
	TotalRequests  int64        `json:"total_requests"`
	TotalBytesIn   int64        `json:"total_bytes_in"`
	TotalBytesOut  int64        `json:"total_bytes_out"`
	P50ResponseMs  float64      `json:"p50_response_ms"`
	P95ResponseMs  float64      `json:"p95_response_ms"`
	P99ResponseMs  float64      `json:"p99_response_ms"`
	StatusCodes    map[int]int64 `json:"status_codes"`
}

// Snapshot returns a MetricsSnapshot for the named service.
// Returns nil if the service has not been seen.
func (c *Collector) Snapshot(name string) *MetricsSnapshot {
	sm := c.GetMetrics(name)
	if sm == nil {
		return nil
	}

	sm.mu.Lock()
	codes := make(map[int]int64, len(sm.StatusCodes))
	for k, v := range sm.StatusCodes {
		codes[k] = v
	}
	sm.mu.Unlock()

	return &MetricsSnapshot{
		ServiceName:    sm.ServiceName,
		ActiveConns:    atomic.LoadInt64(&sm.ActiveConns),
		TotalRequests:  atomic.LoadInt64(&sm.TotalRequests),
		TotalBytesIn:   atomic.LoadInt64(&sm.TotalBytesIn),
		TotalBytesOut:  atomic.LoadInt64(&sm.TotalBytesOut),
		P50ResponseMs:  sm.ResponseTimes.Percentile(0.50),
		P95ResponseMs:  sm.ResponseTimes.Percentile(0.95),
		P99ResponseMs:  sm.ResponseTimes.Percentile(0.99),
		StatusCodes:    codes,
	}
}
