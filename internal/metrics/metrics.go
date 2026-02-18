package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// ServiceMetrics holds per-service traffic counters and timing data.
type ServiceMetrics struct {
	ServiceName   string
	ActiveConns   int64
	TotalRequests int64
	TotalBytesIn  int64
	TotalBytesOut int64

	mu          sync.Mutex
	StatusCodes map[int]int64

	ResponseTimes *RingBuffer
}

func newServiceMetrics(name string) *ServiceMetrics {
	return &ServiceMetrics{
		ServiceName:   name,
		StatusCodes:   make(map[int]int64),
		ResponseTimes: NewRingBuffer(),
	}
}

// Collector aggregates metrics for multiple services.
type Collector struct {
	mu       sync.RWMutex
	services map[string]*ServiceMetrics
}

// NewCollector creates a new, empty Collector.
func NewCollector() *Collector {
	return &Collector{
		services: make(map[string]*ServiceMetrics),
	}
}

// getOrCreate returns the ServiceMetrics for the given name, creating it if necessary.
func (c *Collector) getOrCreate(name string) *ServiceMetrics {
	c.mu.RLock()
	sm, ok := c.services[name]
	c.mu.RUnlock()
	if ok {
		return sm
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check after acquiring write lock.
	sm, ok = c.services[name]
	if ok {
		return sm
	}
	sm = newServiceMetrics(name)
	c.services[name] = sm
	return sm
}

// RecordRequest records a completed request for the named service.
func (c *Collector) RecordRequest(name string, statusCode int, bytesIn, bytesOut int64, duration time.Duration) {
	sm := c.getOrCreate(name)
	atomic.AddInt64(&sm.TotalRequests, 1)
	atomic.AddInt64(&sm.TotalBytesIn, bytesIn)
	atomic.AddInt64(&sm.TotalBytesOut, bytesOut)

	sm.mu.Lock()
	sm.StatusCodes[statusCode]++
	sm.mu.Unlock()

	sm.ResponseTimes.Add(float64(duration.Milliseconds()))
}

// IncrementActiveConns atomically increments the active connection count.
func (c *Collector) IncrementActiveConns(name string) {
	sm := c.getOrCreate(name)
	atomic.AddInt64(&sm.ActiveConns, 1)
}

// DecrementActiveConns atomically decrements the active connection count.
func (c *Collector) DecrementActiveConns(name string) {
	sm := c.getOrCreate(name)
	atomic.AddInt64(&sm.ActiveConns, -1)
}

// GetMetrics returns the ServiceMetrics for the given name, or nil if not found.
func (c *Collector) GetMetrics(name string) *ServiceMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.services[name]
}

// GetAllMetrics returns a snapshot of all service names to their metrics.
func (c *Collector) GetAllMetrics() map[string]*ServiceMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]*ServiceMetrics, len(c.services))
	for k, v := range c.services {
		result[k] = v
	}
	return result
}
