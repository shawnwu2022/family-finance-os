package server

import (
	"net"
	"strings"
	"sync"
	"time"
)

type loginFailureBucket struct {
	count     int
	startedAt time.Time
}

type loginThrottle struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	maxEntries  int
	buckets     map[string]loginFailureBucket
}

func newLoginThrottle(maxFailures int, window time.Duration, maxEntries int) *loginThrottle {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	if maxEntries <= 0 {
		maxEntries = 4096
	}
	return &loginThrottle{
		maxFailures: maxFailures,
		window:      window,
		maxEntries:  maxEntries,
		buckets:     make(map[string]loginFailureBucket),
	}
}

func (l *loginThrottle) Allow(ip, username string, now time.Time) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)
	for _, key := range loginThrottleKeys(ip, username) {
		bucket, ok := l.buckets[key]
		if !ok {
			if len(l.buckets) >= l.maxEntries {
				return false
			}
			continue
		}
		if bucket.count >= l.maxFailures {
			return false
		}
	}
	return true
}

func (l *loginThrottle) RecordFailure(ip, username string, now time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)
	for _, key := range loginThrottleKeys(ip, username) {
		bucket, ok := l.buckets[key]
		if !ok {
			if len(l.buckets) >= l.maxEntries {
				continue
			}
			bucket.startedAt = now
		}
		bucket.count++
		l.buckets[key] = bucket
	}
}

func (l *loginThrottle) RecordSuccess(ip, username string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range loginThrottleKeys(ip, username) {
		delete(l.buckets, key)
	}
}

func (l *loginThrottle) pruneLocked(now time.Time) {
	for key, bucket := range l.buckets {
		if !now.Before(bucket.startedAt.Add(l.window)) {
			delete(l.buckets, key)
		}
	}
}

func loginThrottleKeys(ip, username string) []string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "unknown"
	}
	username = strings.ToLower(strings.TrimSpace(username))
	keys := []string{"ip:" + ip}
	if username != "" {
		keys = append(keys, "user:"+username)
	}
	return keys
}

func loginRemoteHost(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil && strings.TrimSpace(host) != "" {
		return host
	}
	return remoteAddr
}
