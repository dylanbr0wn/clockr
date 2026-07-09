package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWithinLimit(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	lim := New(time.Minute, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		if !lim.Allow("start|ip", 3) {
			t.Fatalf("request %d should allow", i+1)
		}
	}
	if lim.Allow("start|ip", 3) {
		t.Fatal("4th request should deny")
	}
}

func TestAllowResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	lim := New(time.Minute, func() time.Time { return now })

	if !lim.Allow("refresh|ip", 1) {
		t.Fatal("first should allow")
	}
	if lim.Allow("refresh|ip", 1) {
		t.Fatal("second in window should deny")
	}

	now = now.Add(time.Minute)
	if !lim.Allow("refresh|ip", 1) {
		t.Fatal("after window should allow")
	}
	if len(lim.buckets) != 1 {
		t.Fatalf("expected expired keys evicted, got %d buckets", len(lim.buckets))
	}
}

func TestNilLimiterAllows(t *testing.T) {
	var lim *Limiter
	if !lim.Allow("any", 1) {
		t.Fatal("nil limiter must allow")
	}
}

func TestKey(t *testing.T) {
	if got, want := Key("start", "1.2.3.0/24"), "start|1.2.3.0/24"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
