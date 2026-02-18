package metrics

import (
	"math"
	"sync"
	"testing"
)

func TestRingBuffer_AddAndLen(t *testing.T) {
	rb := NewRingBuffer()
	if rb.Len() != 0 {
		t.Fatalf("expected Len()=0, got %d", rb.Len())
	}

	for i := 0; i < 10; i++ {
		rb.Add(float64(i))
	}
	if rb.Len() != 10 {
		t.Fatalf("expected Len()=10, got %d", rb.Len())
	}
}

func TestRingBuffer_Wrap(t *testing.T) {
	rb := NewRingBufferWithCapacity(5)
	for i := 0; i < 8; i++ {
		rb.Add(float64(i))
	}
	if rb.Len() != 5 {
		t.Fatalf("expected Len()=5 after overflow, got %d", rb.Len())
	}
	// Should contain [3,4,5,6,7]
	vals := rb.Values()
	expected := []float64{3, 4, 5, 6, 7}
	for i, v := range vals {
		if v != expected[i] {
			t.Fatalf("Values()[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestRingBuffer_Values_Empty(t *testing.T) {
	rb := NewRingBuffer()
	if vals := rb.Values(); vals != nil {
		t.Fatalf("expected nil for empty buffer, got %v", vals)
	}
}

func TestRingBuffer_Percentile_Empty(t *testing.T) {
	rb := NewRingBuffer()
	if p := rb.Percentile(0.5); p != 0 {
		t.Fatalf("expected 0 for empty buffer, got %f", p)
	}
}

func TestRingBuffer_Percentile_Single(t *testing.T) {
	rb := NewRingBuffer()
	rb.Add(42)
	if p := rb.Percentile(0.5); p != 42 {
		t.Fatalf("expected 42, got %f", p)
	}
}

func TestRingBuffer_Percentile_Known(t *testing.T) {
	rb := NewRingBufferWithCapacity(100)
	for i := 1; i <= 100; i++ {
		rb.Add(float64(i))
	}

	tests := []struct {
		p    float64
		want float64
		tol  float64
	}{
		{0.50, 50.5, 1.0},
		{0.95, 95.05, 1.0},
		{0.99, 99.01, 1.0},
		{0.0, 1.0, 0.01},
		{1.0, 100.0, 0.01},
	}
	for _, tc := range tests {
		got := rb.Percentile(tc.p)
		if math.Abs(got-tc.want) > tc.tol {
			t.Errorf("Percentile(%v) = %f, want ~%f (tol %f)", tc.p, got, tc.want, tc.tol)
		}
	}
}

func TestRingBuffer_Concurrent(t *testing.T) {
	rb := NewRingBuffer()
	var wg sync.WaitGroup

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				rb.Add(float64(base*200 + i))
			}
		}(g)
	}

	// Concurrent reads while writes happen.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = rb.Len()
			_ = rb.Percentile(0.5)
			_ = rb.Values()
		}
	}()

	wg.Wait()

	if rb.Len() != defaultRingCapacity {
		t.Fatalf("expected buffer full at %d, got %d", defaultRingCapacity, rb.Len())
	}
}

func TestNewRingBufferWithCapacity_Invalid(t *testing.T) {
	rb := NewRingBufferWithCapacity(0)
	if rb.capacity != defaultRingCapacity {
		t.Fatalf("expected default capacity for 0, got %d", rb.capacity)
	}
	rb = NewRingBufferWithCapacity(-5)
	if rb.capacity != defaultRingCapacity {
		t.Fatalf("expected default capacity for -5, got %d", rb.capacity)
	}
}
