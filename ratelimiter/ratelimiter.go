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
	duration int64 // Duration in nanoseconds
	window   []int64 // Unix timestamps in nanoseconds
}

// InMemoryRateLimiter implements the RateLimiter interface using a sliding window algorithm
type InMemoryRateLimiter struct {
	mu              sync.RWMutex
	clients         map[string]*clientState
	defaultRequests int
	defaultDuration int64
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
		defaultDuration: defaultDuration.Nanoseconds(),
	}
}

// filterTimestamps removes timestamps older than the cutoff time and returns the filtered window
func filterTimestamps(timestamps []int64, cutoff int64) []int64 {
	filtered := timestamps[:0]
	for _, ts := range timestamps {
		if ts > cutoff {
			filtered = append(filtered, ts)
		}
	}
	return filtered
}

// SetRateLimit configures the rate limit for a specific client
func (rl *InMemoryRateLimiter) SetRateLimit(clientID string, requests int, duration time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if requests < 0 || duration <= 0 {
		requests = 0
	}
	
	durationNanos := duration.Nanoseconds()
	now := time.Now().UnixNano()
	cutoff := now - durationNanos
	
	var existingWindow []int64
	if existingClient, exists := rl.clients[clientID]; exists {
		existingWindow = filterTimestamps(existingClient.window, cutoff)
	}
	
	rl.clients[clientID] = &clientState{
		requests: requests,
		duration: durationNanos,
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
			window:   make([]int64, 0, rl.defaultRequests),
		}
		rl.clients[clientID] = client
	}
	
	now := time.Now().UnixNano()
	cutoff := now - client.duration
	client.window = filterTimestamps(client.window, cutoff)
	
	if len(client.window) >= client.requests {
		return false
	}
	
	client.window = append(client.window, now)
	return true
}
