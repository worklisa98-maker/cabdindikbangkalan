package handlers

import (
	"sync"
	"time"
)

type rateAttempt struct {
	Count int
	First time.Time
}

type userRateLimiter struct {
	limit    int
	window   time.Duration
	mu       sync.Mutex
	attempts map[int]rateAttempt
}

func newUserRateLimiter(limit int, window time.Duration) *userRateLimiter {
	return &userRateLimiter{
		limit:    limit,
		window:   window,
		attempts: make(map[int]rateAttempt),
	}
}

func (r *userRateLimiter) Allow(userID int, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.attempts[userID]
	if !ok || now.Sub(entry.First) > r.window {
		r.attempts[userID] = rateAttempt{Count: 0, First: now}
		return true
	}

	return entry.Count < r.limit
}

func (r *userRateLimiter) RecordFailure(userID int, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.attempts[userID]
	if !ok || now.Sub(entry.First) > r.window {
		r.attempts[userID] = rateAttempt{Count: 1, First: now}
		return
	}

	entry.Count++
	r.attempts[userID] = entry
}

func (r *userRateLimiter) Reset(userID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, userID)
}

var settingsRateLimiter = newUserRateLimiter(5, 10*time.Minute)
