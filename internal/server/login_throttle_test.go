package server

import (
	"testing"
	"time"
)

func TestLoginThrottleBlocksAfterFiveFailuresOnEitherDimension(t *testing.T) {
	t.Parallel()
	limiter := newLoginThrottle(5, 5*time.Minute, 64)
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if !limiter.Allow("203.0.113.10", "Owner", now) {
			t.Fatalf("attempt %d blocked before threshold", i+1)
		}
		limiter.RecordFailure("203.0.113.10", "Owner", now)
	}
	if limiter.Allow("203.0.113.10", "different", now) {
		t.Fatal("same IP allowed after threshold")
	}
	if limiter.Allow("203.0.113.11", "owner", now) {
		t.Fatal("same username allowed after threshold")
	}
}

func TestLoginThrottleExpiresWindowAndSuccessClearsBuckets(t *testing.T) {
	t.Parallel()
	limiter := newLoginThrottle(2, 5*time.Minute, 64)
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	limiter.RecordFailure("203.0.113.10", "owner", now)
	limiter.RecordFailure("203.0.113.10", "owner", now)
	if limiter.Allow("203.0.113.10", "owner", now) {
		t.Fatal("threshold was not enforced")
	}
	if !limiter.Allow("203.0.113.10", "owner", now.Add(5*time.Minute)) {
		t.Fatal("expired failure window still blocked login")
	}
	limiter.RecordFailure("203.0.113.10", "owner", now.Add(5*time.Minute))
	limiter.RecordSuccess("203.0.113.10", "owner")
	if !limiter.Allow("203.0.113.10", "owner", now.Add(5*time.Minute)) {
		t.Fatal("successful password step did not clear throttle buckets")
	}
}

func TestLoginThrottleBoundsMemoryAndFailsClosedWhenSaturated(t *testing.T) {
	t.Parallel()
	limiter := newLoginThrottle(5, 5*time.Minute, 2)
	now := time.Now().UTC()
	limiter.RecordFailure("203.0.113.10", "owner", now)
	if !limiter.Allow("203.0.113.10", "owner", now) {
		t.Fatal("tracked identity below threshold should remain allowed")
	}
	if limiter.Allow("203.0.113.11", "other", now) {
		t.Fatal("new identity should fail closed when throttle table is saturated")
	}
}

func TestLoginThrottleNormalizesRemoteAddress(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"203.0.113.10:49152": "203.0.113.10",
		"[2001:db8::10]:443": "2001:db8::10",
		"203.0.113.10":       "203.0.113.10",
		"":                   "unknown",
	}
	for input, want := range cases {
		if got := loginRemoteHost(input); got != want {
			t.Fatalf("loginRemoteHost(%q)=%q want=%q", input, got, want)
		}
	}
}
