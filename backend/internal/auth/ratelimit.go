package auth

import (
	"sync"
	"time"
)

// Limiter is an in-memory sliding-window limiter (one server instance is enough for the prototype).
type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	window time.Duration
}

func NewLimiter() *Limiter {
	l := &Limiter{hits: map[string][]time.Time{}, window: time.Minute}
	go func() {
		for range time.Tick(5 * time.Minute) {
			l.mu.Lock()
			now := time.Now()
			for k, v := range l.hits {
				if len(v) == 0 || now.Sub(v[len(v)-1]) > l.window {
					delete(l.hits, k)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

// Allow records a hit for key and reports whether it is within max hits per minute.
func (l *Limiter) Allow(key string, max int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-l.window)
	v := l.hits[key]
	i := 0
	for i < len(v) && v[i].Before(cut) {
		i++
	}
	v = v[i:]
	if len(v) >= max {
		l.hits[key] = v
		return false
	}
	l.hits[key] = append(v, now)
	return true
}
