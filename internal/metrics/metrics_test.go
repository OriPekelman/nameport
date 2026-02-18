package metrics

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollector_RecordRequest(t *testing.T) {
	c := NewCollector()
	c.RecordRequest("web", 200, 100, 500, 10*time.Millisecond)
	c.RecordRequest("web", 200, 200, 600, 20*time.Millisecond)
	c.RecordRequest("web", 404, 50, 100, 5*time.Millisecond)

	sm := c.GetMetrics("web")
	if sm == nil {
		t.Fatal("expected non-nil ServiceMetrics")
	}
	if atomic.LoadInt64(&sm.TotalRequests) != 3 {
		t.Fatalf("TotalRequests = %d, want 3", sm.TotalRequests)
	}
	if atomic.LoadInt64(&sm.TotalBytesIn) != 350 {
		t.Fatalf("TotalBytesIn = %d, want 350", sm.TotalBytesIn)
	}
	if atomic.LoadInt64(&sm.TotalBytesOut) != 1200 {
		t.Fatalf("TotalBytesOut = %d, want 1200", sm.TotalBytesOut)
	}

	sm.mu.Lock()
	if sm.StatusCodes[200] != 2 {
		t.Errorf("StatusCodes[200] = %d, want 2", sm.StatusCodes[200])
	}
	if sm.StatusCodes[404] != 1 {
		t.Errorf("StatusCodes[404] = %d, want 1", sm.StatusCodes[404])
	}
	sm.mu.Unlock()
}

func TestCollector_ActiveConns(t *testing.T) {
	c := NewCollector()
	c.IncrementActiveConns("api")
	c.IncrementActiveConns("api")
	c.DecrementActiveConns("api")

	sm := c.GetMetrics("api")
	if sm == nil {
		t.Fatal("expected non-nil ServiceMetrics")
	}
	if atomic.LoadInt64(&sm.ActiveConns) != 1 {
		t.Fatalf("ActiveConns = %d, want 1", sm.ActiveConns)
	}
}

func TestCollector_GetMetrics_Unknown(t *testing.T) {
	c := NewCollector()
	if sm := c.GetMetrics("nonexistent"); sm != nil {
		t.Fatal("expected nil for unknown service")
	}
}

func TestCollector_GetAllMetrics(t *testing.T) {
	c := NewCollector()
	c.RecordRequest("a", 200, 1, 1, time.Millisecond)
	c.RecordRequest("b", 200, 1, 1, time.Millisecond)

	all := c.GetAllMetrics()
	if len(all) != 2 {
		t.Fatalf("expected 2 services, got %d", len(all))
	}
	if _, ok := all["a"]; !ok {
		t.Error("missing service 'a'")
	}
	if _, ok := all["b"]; !ok {
		t.Error("missing service 'b'")
	}
}

func TestCollector_Snapshot(t *testing.T) {
	c := NewCollector()
	for i := 0; i < 100; i++ {
		c.RecordRequest("svc", 200, 10, 20, time.Duration(i+1)*time.Millisecond)
	}
	c.IncrementActiveConns("svc")

	snap := c.Snapshot("svc")
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.ServiceName != "svc" {
		t.Errorf("ServiceName = %q, want %q", snap.ServiceName, "svc")
	}
	if snap.TotalRequests != 100 {
		t.Errorf("TotalRequests = %d, want 100", snap.TotalRequests)
	}
	if snap.ActiveConns != 1 {
		t.Errorf("ActiveConns = %d, want 1", snap.ActiveConns)
	}
	if snap.P50ResponseMs <= 0 {
		t.Errorf("P50ResponseMs should be > 0, got %f", snap.P50ResponseMs)
	}
	if snap.P95ResponseMs <= snap.P50ResponseMs {
		t.Errorf("P95 (%f) should be > P50 (%f)", snap.P95ResponseMs, snap.P50ResponseMs)
	}
}

func TestCollector_Snapshot_Unknown(t *testing.T) {
	c := NewCollector()
	if snap := c.Snapshot("nope"); snap != nil {
		t.Fatal("expected nil snapshot for unknown service")
	}
}

func TestCollector_ConcurrentAccess(t *testing.T) {
	c := NewCollector()
	var wg sync.WaitGroup

	// Multiple goroutines writing to same and different services.
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := "shared"
			if id%2 == 0 {
				name = "even"
			}
			for i := 0; i < 100; i++ {
				c.RecordRequest(name, 200, 1, 1, time.Millisecond)
				c.IncrementActiveConns(name)
				c.DecrementActiveConns(name)
			}
		}(g)
	}

	// Concurrent reads.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = c.GetMetrics("shared")
			_ = c.GetAllMetrics()
			_ = c.Snapshot("shared")
		}
	}()

	wg.Wait()

	// Basic sanity: all requests accounted for.
	all := c.GetAllMetrics()
	var total int64
	for _, sm := range all {
		total += atomic.LoadInt64(&sm.TotalRequests)
	}
	if total != 1000 {
		t.Fatalf("total requests = %d, want 1000", total)
	}
}
