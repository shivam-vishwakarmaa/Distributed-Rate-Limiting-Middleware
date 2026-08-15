package fallback

import (
	"context"
	"sync"
	"time"

	"rate-limiter/internal/limiter"
)

type LocalLimiter struct {
	mu      sync.Mutex
	entries map[string]*localEntry
}

type localEntry struct {
	count     int
	expiresAt time.Time
}

func NewLocalLimiter() *LocalLimiter {
	l := &LocalLimiter{
		entries: make(map[string]*localEntry),
	}
	go l.sweep()
	return l
}

func (l *LocalLimiter) sweep() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for k, v := range l.entries {
			if now.After(v.expiresAt) {
				delete(l.entries, k)
			}
		}
		l.mu.Unlock()
	}
}

func (l *LocalLimiter) Allow(ctx context.Context, req limiter.CheckRequest) (*limiter.Result, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	key := req.RouteID + ":" + req.ClientID

	entry, exists := l.entries[key]
	if !exists || now.After(entry.expiresAt) {
		entry = &localEntry{
			count:     0,
			expiresAt: now.Add(time.Duration(req.WindowSeconds) * time.Second),
		}
		l.entries[key] = entry
	}

	entry.count++
	if entry.count > req.Limit {
		return &limiter.Result{
			Allowed:   false,
			Remaining: 0,
		}, nil
	}

	remaining := req.Limit - entry.count
	return &limiter.Result{
		Allowed:   true,
		Remaining: remaining,
	}, nil
}
