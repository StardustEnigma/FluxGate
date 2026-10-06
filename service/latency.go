package service

import (
	"sync/atomic"
	"time"
)

// latencyStats is a lock-free exponential-bucket histogram for Redis call
// durations. It replaces the previous sync.Mutex + unbounded []time.Duration
// implementation that caused:
//   - Hot-lock contention: 200 concurrent goroutines all serialising through
//     a single mutex after every Redis Eval.
//   - Unbounded heap growth: samples accumulated indefinitely because
//     Snapshot() was never called from production code paths, triggering
//     increasingly expensive GC stop-the-world pauses.
//
// Bucket boundaries (milliseconds): <1  <2  <4  <8  <16  <32  >=32
type latencyStats struct {
	buckets [7]atomic.Int64
}

var redisLatency = &latencyStats{}

// Record classifies d into the appropriate bucket with a single atomic Add.
// No mutex, no allocation, no goroutine parking.
func (s *latencyStats) Record(d time.Duration) {
	ms := d.Milliseconds()
	idx := 0
	for idx < 6 && ms >= (1<<idx) {
		idx++
	}
	s.buckets[idx].Add(1)
}

// Snapshot returns the current bucket counts and resets them to zero.
func (s *latencyStats) Snapshot() [7]int64 {
	var out [7]int64
	for i := range s.buckets {
		out[i] = s.buckets[i].Swap(0)
	}
	return out
}
