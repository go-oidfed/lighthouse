package stats

import (
	"sync"
)

// RingBuffer is a thread-safe circular buffer for storing RequestLog entries.
// It provides non-blocking writes that never fail - if the buffer is full,
// the oldest entries are overwritten.
type RingBuffer struct {
	entries  []*RequestLog
	capacity int
	head     int // next write position
	size     int // current number of entries
	mu       sync.Mutex

	// NotifyThreshold is signaled when the buffer reaches the threshold percentage.
	// It's a buffered channel (size 1) so sends never block.
	NotifyThreshold chan struct{}
	threshold       float64
}

// NewRingBuffer creates a new ring buffer with the given capacity and flush threshold.
// The threshold (0.0 to 1.0) determines when NotifyThreshold is signaled.
func NewRingBuffer(capacity int, threshold float64) *RingBuffer {
	if capacity <= 0 {
		capacity = 10000
	}
	if threshold <= 0 || threshold > 1 {
		threshold = 0.8
	}

	return &RingBuffer{
		entries:         make([]*RequestLog, capacity),
		capacity:        capacity,
		threshold:       threshold,
		NotifyThreshold: make(chan struct{}, 1),
	}
}

// Write adds an entry to the buffer. This operation is non-blocking and always succeeds.
// If the buffer is full, the oldest entry is overwritten.
// Returns true if the threshold was reached (caller may want to trigger a flush).
func (b *RingBuffer) Write(entry *RequestLog) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Write at head position
	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.capacity

	// Update size (capped at capacity)
	if b.size < b.capacity {
		b.size++
	}

	// Check if threshold reached
	thresholdReached := float64(b.size) >= float64(b.capacity)*b.threshold
	if thresholdReached {
		// Non-blocking send to notify channel
		select {
		case b.NotifyThreshold <- struct{}{}:
		default:
			// Already notified, skip
		}
	}

	return thresholdReached
}

// Drain removes and returns all entries from the buffer.
// The buffer is empty after this call.
func (b *RingBuffer) Drain() []*RequestLog {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.size == 0 {
		return nil
	}

	result := make([]*RequestLog, b.size)

	// Calculate start position (oldest entry)
	start := 0
	if b.size == b.capacity {
		// Buffer is full, oldest is at head (which is also next write position)
		start = b.head
	}

	// Copy entries in order from oldest to newest
	for i := 0; i < b.size; i++ {
		idx := (start + i) % b.capacity
		result[i] = b.entries[idx]
		b.entries[idx] = nil // Clear reference for GC
	}

	// Reset buffer
	b.size = 0
	b.head = 0

	return result
}

// Size returns the current number of entries in the buffer.
func (b *RingBuffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size
}

// Capacity returns the maximum capacity of the buffer.
func (b *RingBuffer) Capacity() int {
	return b.capacity
}

// IsFull returns true if the buffer is at capacity.
func (b *RingBuffer) IsFull() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size == b.capacity
}

// FillPercentage returns the current fill level as a percentage (0.0 to 1.0).
func (b *RingBuffer) FillPercentage() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return float64(b.size) / float64(b.capacity)
}
