package ratelimiter

import (
	"sync"
	"time"
)

// RateLimiter interface defines the methods required for rate limiting
type RateLimiter interface {
	Allow(clientID string) bool
	SetRateLimit(clientID string, requests int, duration time.Duration)
}

// clientState holds the rate limiting configuration and state for a client
type clientState struct {
	requests int
	duration time.Duration
	window   []time.Time
}

// InMemoryRateLimiter implements the RateLimiter interface using a sliding window algorithm
type InMemoryRateLimiter struct {
	mu              sync.RWMutex
	clients         map[string]*clientState
	defaultRequests int
	defaultDuration time.Duration
}

// NewRateLimiter creates a new instance of InMemoryRateLimiter with default settings
func NewRateLimiter() RateLimiter {
	return NewRateLimiterWithDefaults(100, time.Minute)
}

// NewRateLimiterWithDefaults creates a new instance with specified defaults
func NewRateLimiterWithDefaults(defaultRequests int, defaultDuration time.Duration) RateLimiter {
	return &InMemoryRateLimiter{
		clients:         make(map[string]*clientState),
		defaultRequests: defaultRequests,
		defaultDuration: defaultDuration,
	}
}

// filterTimestamps removes timestamps older than the cutoff time and returns the filtered window
func filterTimestamps(timestamps []time.Time, cutoff time.Time) []time.Time {
	filtered := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	return filtered
}

// filterTimestampsByDuration returns a new window with timestamps within the given duration from now
func filterTimestampsByDuration(timestamps []time.Time, duration time.Duration) []time.Time {
	now := time.Now()
	cutoff := now.Add(-duration)
	return filterTimestamps(timestamps, cutoff)
}

// SetRateLimit configures the rate limit for a specific client
func (rl *InMemoryRateLimiter) SetRateLimit(clientID string, requests int, duration time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if requests < 0 || duration <= 0 {
		requests = 0
	}
	
	var existingWindow []time.Time
	if existingClient, exists := rl.clients[clientID]; exists {
		existingWindow = filterTimestampsByDuration(existingClient.window, duration)
	}
	
	rl.clients[clientID] = &clientState{
		requests: requests,
		duration: duration,
		window:   existingWindow,
	}
}

// Allow checks if a request from a client should be allowed based on their rate limit
func (rl *InMemoryRateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	client, exists := rl.clients[clientID]
	if !exists {
		client = &clientState{
			requests: rl.defaultRequests,
			duration: rl.defaultDuration,
			window:   make([]time.Time, 0, rl.defaultRequests),
		}
		rl.clients[clientID] = client
	}
	
	client.window = filterTimestampsByDuration(client.window, client.duration)
	
	if len(client.window) >= client.requests {
		return false
	}
	
	client.window = append(client.window, time.Now())
	return true
}
