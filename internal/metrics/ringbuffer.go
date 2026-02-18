package metrics

import (
	"math"
	"sort"
	"sync"
)

const defaultRingCapacity = 1000

// RingBuffer is a fixed-size, thread-safe ring buffer for float64 values.
// When the buffer is full, new values overwrite the oldest entries.
type RingBuffer struct {
	mu       sync.Mutex
	data     []float64
	pos      int
	count    int
	capacity int
}

// NewRingBuffer creates a new RingBuffer with the default capacity (1000).
func NewRingBuffer() *RingBuffer {
	return NewRingBufferWithCapacity(defaultRingCapacity)
}

// NewRingBufferWithCapacity creates a new RingBuffer with the specified capacity.
func NewRingBufferWithCapacity(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = defaultRingCapacity
	}
	return &RingBuffer{
		data:     make([]float64, capacity),
		capacity: capacity,
	}
}

// Add inserts a value into the ring buffer, overwriting the oldest value
// if the buffer is full.
func (rb *RingBuffer) Add(v float64) {
	rb.mu.Lock()
	rb.data[rb.pos] = v
	rb.pos = (rb.pos + 1) % rb.capacity
	if rb.count < rb.capacity {
		rb.count++
	}
	rb.mu.Unlock()
}

// Len returns the number of values currently stored in the buffer.
func (rb *RingBuffer) Len() int {
	rb.mu.Lock()
	n := rb.count
	rb.mu.Unlock()
	return n
}

// Values returns a copy of all stored values in insertion order.
func (rb *RingBuffer) Values() []float64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return nil
	}

	result := make([]float64, rb.count)
	if rb.count < rb.capacity {
		copy(result, rb.data[:rb.count])
	} else {
		// Buffer is full; oldest element is at rb.pos
		n := copy(result, rb.data[rb.pos:])
		copy(result[n:], rb.data[:rb.pos])
	}
	return result
}

// Percentile computes the p-th percentile (0.0–1.0) of the stored values
// using linear interpolation. Returns 0 if the buffer is empty.
func (rb *RingBuffer) Percentile(p float64) float64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return 0
	}

	sorted := make([]float64, rb.count)
	if rb.count < rb.capacity {
		copy(sorted, rb.data[:rb.count])
	} else {
		copy(sorted, rb.data)
	}
	sort.Float64s(sorted)

	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}

	// Use the "nearest rank" method with linear interpolation.
	rank := p * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
