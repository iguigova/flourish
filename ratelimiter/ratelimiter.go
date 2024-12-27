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
	mu       sync.Mutex  // Individual lock for this client
	requests int
	duration int64      // Duration in nanoseconds
	window   []int64    // Unix timestamps in nanoseconds
}

// InMemoryRateLimiter implements the RateLimiter interface using a sliding window algorithm
type InMemoryRateLimiter struct {
	mu              sync.RWMutex  // Only used when accessing the clients map
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

// getOrCreateClient retrieves an existing client or creates a new one
func (rl *InMemoryRateLimiter) getOrCreateClient(clientID string) *clientState {
	rl.mu.RLock()
	client, exists := rl.clients[clientID]
	rl.mu.RUnlock()
	
	if exists {
		return client
	}
	
	// Need to create new client - acquire write lock
	rl.mu.Lock()
	// Double-check pattern in case another goroutine created it
	if client, exists = rl.clients[clientID]; exists {
		rl.mu.Unlock()
		return client
	}
	
	client = &clientState{
		requests: rl.defaultRequests,
		duration: rl.defaultDuration,
		window:   make([]int64, 0, rl.defaultRequests),
	}
	rl.clients[clientID] = client
	rl.mu.Unlock()
	return client
}

// filterTimestamps removes timestamps older than the cutoff time and returns the filtered window
// timestamps are guaranteed to be in ascending order since they're added using time.Now()
func filterTimestamps(timestamps []int64, cutoff int64) []int64 {
    // Find first timestamp that's newer than cutoff
    i := 0
    for ; i < len(timestamps); i++ {
        if timestamps[i] > cutoff {
            break
        }
    }
    
    // If i == len(timestamps), all timestamps are older than cutoff
    // If i == 0, all timestamps are newer than cutoff
    // Otherwise i is the index of first timestamp we want to keep
    
    // Use same underlying array but slice from i onwards
    // This avoids copying timestamps individually
    return timestamps[i:]
}

// SetRateLimit configures the rate limit for a specific client
func (rl *InMemoryRateLimiter) SetRateLimit(clientID string, requests int, duration time.Duration) {
	if requests < 0 || duration <= 0 {
		requests = 0
	}
	
	client := rl.getOrCreateClient(clientID)
	client.mu.Lock()
	defer client.mu.Unlock()
	
	durationNanos := duration.Nanoseconds()
	now := time.Now().UnixNano()
	cutoff := now - durationNanos
	
	client.window = filterTimestamps(client.window, cutoff)
	client.requests = requests
	client.duration = durationNanos
}

// Allow checks if a request from a client should be allowed based on their rate limit
func (rl *InMemoryRateLimiter) Allow(clientID string) bool {
	client := rl.getOrCreateClient(clientID)
	client.mu.Lock()
	defer client.mu.Unlock()
	
	now := time.Now().UnixNano()
	cutoff := now - client.duration
	client.window = filterTimestamps(client.window, cutoff)
	
	if len(client.window) >= client.requests {
		return false
	}
	
	client.window = append(client.window, now)
	return true
}

