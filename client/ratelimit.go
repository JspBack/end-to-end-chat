package client

import (
	"sync"
	"time"
)

type msgLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	tokens int
	last   time.Time
}

func newMsgLimiter(limit int, window time.Duration) *msgLimiter {
	return &msgLimiter{
		limit:  limit,
		window: window,
		tokens: limit,
		last:   time.Now(),
	}
}

func (l *msgLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.last)

	interval := l.window / time.Duration(l.limit)
	if interval == 0 {
		interval = time.Nanosecond
	}

	refill := int(elapsed / interval)
	if refill > 0 {
		l.last = l.last.Add(time.Duration(refill) * interval)
		l.tokens += refill
		if l.tokens > l.limit {
			l.tokens = l.limit
		}
	}

	if l.tokens > 0 {
		l.tokens--
		return true
	}
	return false
}
