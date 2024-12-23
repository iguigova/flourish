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

// SetRateLimit configures the rate limit for a specific client
func (rl *InMemoryRateLimiter) SetRateLimit(clientID string, requests int, duration time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Disable rate limiting when input is invalid
	if requests < 0 || duration <= 0 {
		requests = 0
	}

	rl.clients[clientID] = &clientState{
		requests: requests,
		duration: duration,
		window:   make([]time.Time, 0, requests),
	}
}

// Allow checks if a request from a client should be allowed based on their rate limit
func (rl *InMemoryRateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[clientID]
	if !exists {
		// Create new client with default rate limit
		client = &clientState{
			requests: rl.defaultRequests,
			duration: rl.defaultDuration,
			window:   make([]time.Time, 0, rl.defaultRequests),
		}
		rl.clients[clientID] = client
	}

	now := time.Now()
	windowStart := now.Add(-client.duration)

	// Remove timestamps outside the current window
	newWindow := client.window[:0]
	for _, ts := range client.window {
		if ts.After(windowStart) {
			newWindow = append(newWindow, ts)
		}
	}
	client.window = newWindow

	// Check if we're at the limit
	if len(client.window) >= client.requests {
		return false
	}

	// Add current request timestamp
	client.window = append(client.window, now)
	return true
}
