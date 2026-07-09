// Package ratelimit provides a fixed-window in-process rate limiter for the
// OAuth broker. Suitable for a single-replica deployment.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter counts events in fixed windows keyed by surface + dimension.
type Limiter struct {
	mu      sync.Mutex
	clock   func() time.Time
	window  time.Duration
	buckets map[string]*bucket
}

type bucket struct {
	windowStart time.Time
	count       int
}

// New returns a limiter with the given window size. A nil clock uses time.Now.
func New(window time.Duration, clock func() time.Time) *Limiter {
	if window <= 0 {
		window = time.Minute
	}
	if clock == nil {
		clock = time.Now
	}
	return &Limiter{
		clock:   clock,
		window:  window,
		buckets: make(map[string]*bucket),
	}
}

// Allow reports whether one more event is permitted for key under limit.
// On allow, the counter is incremented. Expired buckets are evicted lazily.
func (l *Limiter) Allow(key string, limit int) bool {
	if l == nil || limit <= 0 {
		return true
	}
	now := l.clock()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.evictExpired(now)

	b := l.buckets[key]
	if b == nil || now.Sub(b.windowStart) >= l.window {
		l.buckets[key] = &bucket{windowStart: now, count: 1}
		return true
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

func (l *Limiter) evictExpired(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.windowStart) >= l.window {
			delete(l.buckets, key)
		}
	}
}

// Key joins surface and dimension into a stable limiter key.
func Key(surface, dimension string) string {
	return surface + "|" + dimension
}
