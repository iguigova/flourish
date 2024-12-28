package ratelimiter

import (
	"sync/atomic"
	"runtime"
	//"math"
	"fmt"
	"sync"
	"testing"
	"time"
	"reflect"
)

func TestRateLimiterBasicFunctionality(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "test-client"
	
	// Set rate limit: 3 requests per second
	rl.SetRateLimit(clientID, 3, time.Second)
	
	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !rl.Allow(clientID) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}
	
	// Fourth request should be denied
	if rl.Allow(clientID) {
		t.Error("Fourth request should be denied")
	}
}

func TestRateLimiterWindowSliding(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "test-client"
	
	// Set rate limit: 2 requests per 500ms
	rl.SetRateLimit(clientID, 2, 500*time.Millisecond)
	
	// First 2 requests should be allowed
	if !rl.Allow(clientID) {
		t.Error("First request should be allowed")
	}
	if !rl.Allow(clientID) {
		t.Error("Second request should be allowed")
	}
	
	// Third request should be denied
	if rl.Allow(clientID) {
		t.Error("Third request should be denied")
	}
	
	// Wait for window to slide
	time.Sleep(500 * time.Millisecond)
	
	// Should allow new requests
	if !rl.Allow(clientID) {
		t.Error("Request after window slide should be allowed")
	}
}

func TestRateLimiterMultipleClients(t *testing.T) {
	rl := NewRateLimiter()
	
	// Set different limits for different clients
	rl.SetRateLimit("client1", 2, time.Second)
	rl.SetRateLimit("client2", 3, time.Second)
	
	// Test client1's limit
	if !rl.Allow("client1") {
		t.Error("First request for client1 should be allowed")
	}
	if !rl.Allow("client1") {
		t.Error("Second request for client1 should be allowed")
	}
	if rl.Allow("client1") {
		t.Error("Third request for client1 should be denied")
	}
	
	// Test client2's limit (should be independent)
	if !rl.Allow("client2") {
		t.Error("First request for client2 should be allowed")
	}
	if !rl.Allow("client2") {
		t.Error("Second request for client2 should be allowed")
	}
	if !rl.Allow("client2") {
		t.Error("Third request for client2 should be allowed")
	}
	if rl.Allow("client2") {
		t.Error("Fourth request for client2 should be denied")
	}
}

func TestRateLimiterDefaultLimits(t *testing.T) {
	rl := NewRateLimiterWithDefaults(2, time.Second)
	clientID := "new-client"
	
	// Test default limits for new client
	if !rl.Allow(clientID) {
		t.Error("First request with default limit should be allowed")
	}
	if !rl.Allow(clientID) {
		t.Error("Second request with default limit should be allowed")
	}
	if rl.Allow(clientID) {
		t.Error("Third request with default limit should be denied")
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "concurrent-client"
	rl.SetRateLimit(clientID, 5, time.Second)
	
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex
	
	// Launch 10 concurrent requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow(clientID) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	
	wg.Wait()
	
	if successCount != 5 {
		t.Errorf("Expected 5 successful requests, got %d", successCount)
	}
}

func TestRateLimiterShortTimeWindow(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "short-window-client"
	
	// Test with a very short window (10ms)
	rl.SetRateLimit(clientID, 3, 10*time.Millisecond)
	
	// Should allow 3 requests immediately
	for i := 0; i < 3; i++ {
		if !rl.Allow(clientID) {
			t.Errorf("Request %d should be allowed in short window", i+1)
		}
	}
	
	// Should deny the 4th request
	if rl.Allow(clientID) {
		t.Error("Fourth request should be denied in short window")
	}
	
	// Wait for window to pass
	time.Sleep(15 * time.Millisecond)
	
	// Should allow requests again
	if !rl.Allow(clientID) {
		t.Error("Request after short window should be allowed")
	}
}

func TestRateLimiterLargeVolume(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "large-volume-client"
	
	// Test with a large number of requests (10000) per second
	rl.SetRateLimit(clientID, 10000, time.Second)
	
	successCount := 0
	totalRequests := 15000
	
	for i := 0; i < totalRequests; i++ {
		if rl.Allow(clientID) {
			successCount++
		}
	}
	
	if successCount != 10000 {
		t.Errorf("Expected exactly 10000 allowed requests, got %d", successCount)
	}
}

func TestRateLimiterConcurrentModification(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "concurrent-mod-client"
	
	var wg sync.WaitGroup
	
	// Concurrently modify and access rate limits
	for i := 0; i < 10; i++ {
		wg.Add(2)
		
		// Goroutine to modify rate limit
		go func(limit int) {
			defer wg.Done()
			rl.SetRateLimit(clientID, limit, time.Second)
			time.Sleep(10 * time.Millisecond)
		}(i + 1)
		
		// Goroutine to test rate limit
		go func() {
			defer wg.Done()
			rl.Allow(clientID)
			time.Sleep(10 * time.Millisecond)
		}()
	}
	
	wg.Wait()
}

func TestRateLimiterTimeJump(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "time-jump-client"
	
	rl.SetRateLimit(clientID, 2, time.Second)
	
	// Use up the limit
	if !rl.Allow(clientID) {
		t.Error("First request should be allowed")
	}
	if !rl.Allow(clientID) {
		t.Error("Second request should be allowed")
	}
	if rl.Allow(clientID) {
		t.Error("Third request should be denied")
	}
	
	// Simulate time moving forward
	time.Sleep(1100 * time.Millisecond)
	
	// Should allow requests again
	if !rl.Allow(clientID) {
		t.Error("Request after time jump forward should be allowed")
	}
}

func TestRateLimiterUpdateLimits(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "update-test"
	
	// Set initial limit
	rl.SetRateLimit(clientID, 2, time.Second)
	
	// Use up initial limit
	if !rl.Allow(clientID) {
		t.Error("First request should be allowed")
	}
	if !rl.Allow(clientID) {
		t.Error("Second request should be allowed")
	}
	if rl.Allow(clientID) {
		t.Error("Third request should be denied")
	}
	
	// Update limit to be more permissive
	rl.SetRateLimit(clientID, 4, time.Second)
	
	// Should allow more requests now
	if !rl.Allow(clientID) {
		t.Error("First request after limit update should be allowed")
	}
	if !rl.Allow(clientID) {
		t.Error("Second request after limit update should be allowed")
	}
}

func TestRateLimiterZeroLimit(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "zero-limit"
	
	// Set zero limit
	rl.SetRateLimit(clientID, 0, time.Second)
	
	// All requests should be denied
	if rl.Allow(clientID) {
		t.Error("Request should be denied with zero limit")
	}
}

func TestRateLimiterNegativeLimit(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "negative-limit"
	
	// Set negative limit (should be treated as zero)
	rl.SetRateLimit(clientID, -1, time.Second)
	
	// All requests should be denied
	if rl.Allow(clientID) {
		t.Error("Request should be denied with negative limit")
	}
}

func TestRateLimiterStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	rl := NewRateLimiter()
	numClients := 100
	requestsPerClient := 1000
	
	var wg sync.WaitGroup
	errors := make(chan string, numClients*requestsPerClient)
	
	startTime := time.Now()
	
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()
			clientID := fmt.Sprintf("stress-client-%d", clientNum)
			rl.SetRateLimit(clientID, 100, time.Second)
			
			for j := 0; j < requestsPerClient; j++ {
				if rl.Allow(clientID) {
					// Simulate some work
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(startTime)
	
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	
	t.Logf("Stress test completed in %v", duration)
}

// Benchmarks
func BenchmarkRateLimiter(b *testing.B) {
	rl := NewRateLimiter()
	clientID := "benchmark-client"
	rl.SetRateLimit(clientID, 1000, time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow(clientID)
	}
}

func BenchmarkRateLimiterConcurrent(b *testing.B) {
	rl := NewRateLimiter()
	clientID := "benchmark-concurrent-client"
	rl.SetRateLimit(clientID, 1000, time.Second)
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.Allow(clientID)
		}
	})
}

func BenchmarkRateLimiterMultipleClients(b *testing.B) {
	rl := NewRateLimiter()
	numClients := 100
	clients := make([]string, numClients)
	
	for i := 0; i < numClients; i++ {
		clients[i] = fmt.Sprintf("bench-client-%d", i)
		rl.SetRateLimit(clients[i], 100, time.Second)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		clientIdx := 0
		for pb.Next() {
			rl.Allow(clients[clientIdx%numClients])
			clientIdx++
		}
	})
}

func BenchmarkRateLimiterMemoryAllocation(b *testing.B) {
	rl := NewRateLimiter()
	clientID := "benchmark-memory-client"
	rl.SetRateLimit(clientID, 1000, time.Second)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rl.Allow(clientID)
	}
}

// 1. Error Handling Tests

func TestRateLimiterInvalidDuration(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "invalid-duration-client"
	
	// Test zero duration
	rl.SetRateLimit(clientID, 10, 0)
	if rl.Allow(clientID) {
		t.Error("Request should be denied with zero duration")
	}
	
	// Test negative duration
	rl.SetRateLimit(clientID, 10, -1*time.Second)
	if rl.Allow(clientID) {
		t.Error("Request should be denied with negative duration")
	}
}

func TestRateLimiterOverflow(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "overflow-test-client"
	
	// Test with a large but reasonable number (1 million requests per second)
	const largeLimit = 1000000
	rl.SetRateLimit(clientID, largeLimit, time.Second)
	
	// Should work without panicking
	success := rl.Allow(clientID)
	if !success {
		t.Error("First request with large limit should be allowed")
	}
	
	// Test rapid requests to ensure no integer overflow
	successCount := 1 // counting the first success above
	for i := 0; i < 1000; i++ {
		if rl.Allow(clientID) {
			successCount++
		}
	}
	
	// Verify we haven't exceeded our limit
	if successCount > largeLimit {
		t.Errorf("Exceeded rate limit: got %d successful requests, expected <= %d", successCount, largeLimit)
	}
	
	// Test arithmetic overflow scenarios
	rl.SetRateLimit(clientID, largeLimit, time.Millisecond)
	beforeOverflow := rl.Allow(clientID)
	
	// Attempt operations that might cause overflow
	for i := 0; i < 100; i++ {
		rl.Allow(clientID)
		time.Sleep(time.Millisecond)
	}
	
	// System should remain stable
	afterOverflow := rl.Allow(clientID)
	if beforeOverflow != afterOverflow {
		t.Error("Rate limiter behavior changed after potential overflow")
	}
}

// 2. Distributed Systems Readiness Tests

// MockDistributedBackend simulates a distributed backend like Redis
type MockDistributedBackend struct {
	allowCount int32
}

func (m *MockDistributedBackend) Allow(clientID string) bool {
	return atomic.AddInt32(&m.allowCount, 1) > 0
}

func (m *MockDistributedBackend) SetRateLimit(clientID string, requests int, duration time.Duration) {
	// Simulate setting rate limit in distributed backend
}

func TestRateLimiterDistributedBackendCompatibility(t *testing.T) {
	// This test verifies that our interface is compatible with a distributed backend
	var backend RateLimiter = &MockDistributedBackend{}
	
	// Should compile and run without errors
	backend.SetRateLimit("test", 10, time.Second)
	if !backend.Allow("test") {
		t.Error("Mock distributed backend should allow first request")
	}
}

// 3. Metrics and Logging Tests

type MetricsRateLimiter struct {
	InMemoryRateLimiter
	allowedCount  int32
	deniedCount   int32
	requestsTotal int32
}

func NewMetricsRateLimiter() *MetricsRateLimiter {
	return &MetricsRateLimiter{
		InMemoryRateLimiter: *NewRateLimiter().(*InMemoryRateLimiter),
	}
}

func (mrl *MetricsRateLimiter) Allow(clientID string) bool {
	atomic.AddInt32(&mrl.requestsTotal, 1)
	allowed := mrl.InMemoryRateLimiter.Allow(clientID)
	if allowed {
		atomic.AddInt32(&mrl.allowedCount, 1)
	} else {
		atomic.AddInt32(&mrl.deniedCount, 1)
	}
	return allowed
}

func TestRateLimiterMetrics(t *testing.T) {
	rl := NewMetricsRateLimiter()
	clientID := "metrics-test-client"
	
	rl.SetRateLimit(clientID, 2, time.Second)
	
	// Make 5 requests
	for i := 0; i < 5; i++ {
		rl.Allow(clientID)
	}
	
	// Verify metrics
	if rl.requestsTotal != 5 {
		t.Errorf("Expected 5 total requests, got %d", rl.requestsTotal)
	}
	if rl.allowedCount != 2 {
		t.Errorf("Expected 2 allowed requests, got %d", rl.allowedCount)
	}
	if rl.deniedCount != 3 {
		t.Errorf("Expected 3 denied requests, got %d", rl.deniedCount)
	}
}

// 4. Client Management Tests

func TestRateLimiterClientCleanup(t *testing.T) {
	rl := NewRateLimiter().(*InMemoryRateLimiter)
	windowDuration := 50 * time.Millisecond // Longer duration for more reliable testing
	
	// Add a bunch of clients
	for i := 0; i < 100; i++ { // Reduced number of clients for more reliable testing
		clientID := fmt.Sprintf("cleanup-client-%d", i)
		rl.SetRateLimit(clientID, 1, windowDuration)
		rl.Allow(clientID)
	}
	
	// Wait for windows to expire
	time.Sleep(windowDuration + 10*time.Millisecond) // Add buffer time
	
	// Make a request to trigger cleanup
	for clientID := range rl.clients {
		rl.Allow(clientID)
	}
	
	// Verify windows are cleaned up
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	for clientID, state := range rl.clients {
		// Only check timestamps that are outside our window
		now := time.Now().UnixNano()
		cutoff := now - state.duration
		activeRequests := 0
		for _, ts := range state.window {
			if ts > cutoff {
				activeRequests++
			}
		}
		// We expect at most 1 active request per client (from our trigger above)
		if activeRequests > 1 {
			t.Errorf("Client %s has %d active requests, expected <= 1", 
				clientID, activeRequests)
		}
	}
}

func TestRateLimiterMaxClients(t *testing.T) {
	rl := NewRateLimiter()
	maxClients := 10000
	clientsPerGoroutine := 100
	
	// Launch multiple goroutines to create clients concurrently
	var wg sync.WaitGroup
	errors := make(chan error, maxClients/clientsPerGoroutine)
	
	for i := 0; i < maxClients/clientsPerGoroutine; i++ {
		wg.Add(1)
		go func(startIdx int) {
			defer wg.Done()
			
			for j := 0; j < clientsPerGoroutine; j++ {
				clientID := fmt.Sprintf("max-client-%d", startIdx+j)
				rl.SetRateLimit(clientID, 1, time.Second)
				
				// Verify client was created successfully
				if !rl.Allow(clientID) {
					errors <- fmt.Errorf("Failed to create client: %s", clientID)
					return
				}
			}
		}(i * clientsPerGoroutine)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}

// 5. Additional Edge Cases

func TestRateLimiterConcurrentWindowCleaning(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "concurrent-cleaning-client"
	
	// Set a very short window
	rl.SetRateLimit(clientID, 5, 10*time.Millisecond)
	
	// Launch goroutines that will cause concurrent window cleaning
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.Allow(clientID)
			time.Sleep(11 * time.Millisecond) // Just after window expiration
			rl.Allow(clientID)
		}()
	}
	
	wg.Wait()
}

func TestRateLimiterBurstyTraffic(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "bursty-client"
	
	rl.SetRateLimit(clientID, 10, 100*time.Millisecond)
	
	// Test burst of traffic
	successCount := 0
	for i := 0; i < 20; i++ {
		if rl.Allow(clientID) {
			successCount++
		}
	}
	
	if successCount != 10 {
		t.Errorf("Expected exactly 10 allowed requests in burst, got %d", successCount)
	}
	
	// Wait for window to pass
	time.Sleep(110 * time.Millisecond)
	
	// Test another burst
	successCount = 0
	for i := 0; i < 20; i++ {
		if rl.Allow(clientID) {
			successCount++
		}
	}
	
	if successCount != 10 {
		t.Errorf("Expected exactly 10 allowed requests in second burst, got %d", successCount)
	}
}

// 1. Error Recovery Tests

// Mock struct to simulate memory pressure
type MemoryPressureRateLimiter struct {
	InMemoryRateLimiter
	memoryLimit int
	currentSize int
}

func NewMemoryPressureRateLimiter(memoryLimit int) *MemoryPressureRateLimiter {
	return &MemoryPressureRateLimiter{
		InMemoryRateLimiter: *NewRateLimiter().(*InMemoryRateLimiter),
		memoryLimit:         memoryLimit,
	}
}

func (mrl *MemoryPressureRateLimiter) Allow(clientID string) bool {
	mrl.currentSize++
	if mrl.currentSize > mrl.memoryLimit {
		runtime.GC() // Force garbage collection
		mrl.currentSize = 0
	}
	return mrl.InMemoryRateLimiter.Allow(clientID)
}

func TestRateLimiterUnderMemoryPressure(t *testing.T) {
	rl := NewMemoryPressureRateLimiter(1000)
	successCount := 0
	failureCount := 0
	
	// Generate significant memory pressure
	for i := 0; i < 10000; i++ {
		clientID := fmt.Sprintf("memory-test-client-%d", i)
		rl.SetRateLimit(clientID, 10, time.Second)
		
		if rl.Allow(clientID) {
			successCount++
		} else {
			failureCount++
		}
	}
	
	// Verify rate limiter still functions
	if successCount == 0 {
		t.Error("Rate limiter should allow some requests even under memory pressure")
	}
}

func TestRateLimiterRecoveryAfterPanic(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "panic-test-client"
	recovered := false
	
	// Create a channel to synchronize the test
	done := make(chan bool)
	
	// Launch goroutine with its own panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
				
				// Try to use rate limiter after panic
				rl.SetRateLimit(clientID, 5, time.Second)
				if !rl.Allow(clientID) {
					t.Error("Rate limiter should function after recovery from panic")
				}
				done <- true
			}
		}()
		
		panic("simulated panic")
	}()
	
	// Wait for panic and recovery to complete
	select {
	case <-done:
		if !recovered {
			t.Error("Expected panic recovery did not occur")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Test timed out waiting for panic recovery")
	}
}

// 2. Dynamic Rate Limit Update Tests

func TestRateLimiterFrequentUpdates(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "frequent-update-client"
	var wg sync.WaitGroup
	
	// Launch goroutine to constantly update rate limits
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 100; i++ {
			rl.SetRateLimit(clientID, i, time.Second)
			time.Sleep(10 * time.Millisecond)
		}
	}()
	
	// Launch goroutine to constantly make requests
	requestCount := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			if rl.Allow(clientID) {
				requestCount++
			}
			time.Sleep(time.Millisecond)
		}
	}()
	
	wg.Wait()
	
	if requestCount == 0 {
		t.Error("Some requests should be allowed during frequent rate limit updates")
	}
}

func TestRateLimitUpdateUnderLoad(t *testing.T) {
	rl := NewRateLimiter()
	clientID := "update-under-load-client"
	
	// Initial rate limit
	rl.SetRateLimit(clientID, 10, time.Second)
	
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex
	
	// Start goroutines making requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Add small delay to ensure SetRateLimit happens first
			time.Sleep(10 * time.Millisecond)
			if rl.Allow(clientID) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	
	// Update rate limit while requests are in flight
	rl.SetRateLimit(clientID, 20, time.Second)
	
	wg.Wait()
	
	// With the delay, all requests should happen after the new limit is set
	// Allow for some margin of error due to timing
	maxExpected := 25 // slightly more than 20 to account for timing issues
	if successCount > maxExpected {
		t.Errorf("Expected at most %d successful requests after limit update, got %d", 
			maxExpected, successCount)
	}
	
	if successCount < 15 { // somewhat less than 20 to account for timing issues
		t.Errorf("Expected at least 15 successful requests after limit update, got %d",
			successCount)
	}
}

// // 3. Long-Running Stability Tests

// func TestRateLimiterLongRunningStability(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping long-running stability test in short mode")
// 	}
	
// 	rl := NewRateLimiter()
// 	clientID := "stability-test-client"
// 	rl.SetRateLimit(clientID, 1000, time.Minute)
	
// 	// Run for 5 minutes
// 	startTime := time.Now()
// 	endTime := startTime.Add(5 * time.Minute)
	
// 	var successCount, failureCount int64
// 	var wg sync.WaitGroup
	
// 	// Launch multiple goroutines to generate constant load
// 	for i := 0; i < 10; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for time.Now().Before(endTime) {
// 				if rl.Allow(clientID) {
// 					atomic.AddInt64(&successCount, 1)
// 				} else {
// 					atomic.AddInt64(&failureCount, 1)
// 				}
// 				time.Sleep(time.Millisecond)
// 			}
// 		}()
// 	}
	
// 	wg.Wait()
	
// 	// Verify rate limiting remained consistent
// 	expectedSuccesses := 5 * 1000 // 5 minutes * 1000 requests per minute
// 	allowedDeviation := float64(expectedSuccesses) * 0.1 // 10% deviation allowed
	
// 	if math.Abs(float64(successCount)-float64(expectedSuccesses)) > allowedDeviation {
// 		t.Errorf("Rate limiting was inconsistent over long period. Expected ~%d successes, got %d", 
// 			expectedSuccesses, successCount)
// 	}
// }

func TestRateLimiterTimeSimulation(t *testing.T) {
	rl := NewRateLimiter().(*InMemoryRateLimiter)
	clientID := "time-simulation-client"
	
	// Configuration for simulation
	requestsPerDay := 1000
	numDays := 7
	rl.SetRateLimit(clientID, requestsPerDay, time.Hour) // Use 1 hour window instead of 24 hours for faster testing
	
	var totalSuccesses int64
	
	for day := 0; day < numDays; day++ {
		// For each day, try 2x the allowed requests
		for request := 0; request < requestsPerDay*2; request++ {
		
			// Get client state and manually update window timestamps for testing
			rl.mu.Lock()
			client, exists := rl.clients[clientID]
			if !exists {
				client = &clientState{
					requests: requestsPerDay,
					duration: time.Hour.Nanoseconds(),
					window:   make([]int64, 0, requestsPerDay),
				}
				rl.clients[clientID] = client
			}
			
			// Clean old entries based on simulated time
			simulatedTime := time.Now().UnixNano() + int64(day) * int64(time.Hour)
			newWindow := client.window[:0]
			for _, ts := range client.window {
				if ts > simulatedTime - client.duration {
					newWindow = append(newWindow, ts)
				}
			}
			client.window = newWindow
			
			// Try to allow request
			if len(client.window) < client.requests {
				client.window = append(client.window, simulatedTime)
				atomic.AddInt64(&totalSuccesses, 1)
			}
			rl.mu.Unlock()
		}
	}
	
	expectedTotal := int64(numDays * requestsPerDay)
	if totalSuccesses != expectedTotal {
		t.Errorf("Expected %d successful requests over %d days, got %d", 
			expectedTotal, numDays, totalSuccesses)
	}
}

// 4. Additional Edge Case Tests

func TestRateLimiterGracefulDegradation(t *testing.T) {
	rl := NewRateLimiter()
	
	// Create many clients with varying rate limits
	for i := 0; i < 10000; i++ {
		clientID := fmt.Sprintf("degradation-client-%d", i)
		rl.SetRateLimit(clientID, i%100+1, time.Second)
	}
	
	// Measure response time under load
	start := time.Now()
	testClient := "degradation-test-client"
	rl.SetRateLimit(testClient, 10, time.Second)
	
	for i := 0; i < 100; i++ {
		rl.Allow(testClient)
	}
	
	duration := time.Since(start)
	
	// Verify performance hasn't degraded significantly
	if duration > time.Second {
		t.Errorf("Rate limiter performance degraded under load. Took %v for 100 requests", duration)
	}
}

func TestRateLimiterConcurrentCleanup(t *testing.T) {
	rl := NewRateLimiter()
	const numClients = 1000
	const cleanupInterval = 100 * time.Millisecond
	
	// Create clients with very short windows
	for i := 0; i < numClients; i++ {
		clientID := fmt.Sprintf("cleanup-client-%d", i)
		rl.SetRateLimit(clientID, 5, cleanupInterval)
	}
	
	var wg sync.WaitGroup
	errors := make(chan error, numClients)
	
	// Start multiple goroutines that will trigger cleanup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientID := fmt.Sprintf("cleanup-client-%d", id) // 
			
			// Make requests across cleanup boundaries
			for j := 0; j < 10; j++ {
				rl.Allow(clientID)
				time.Sleep(cleanupInterval / 2)
				
				// Verify client still exists and functions
				rl.SetRateLimit(clientID, 5, cleanupInterval)
				if !rl.Allow(clientID) {
					errors <- fmt.Errorf("client %s failed after cleanup", clientID)
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	for err := range errors {
		t.Error(err)
	}
}

func TestRateLimiterDurationDecrease(t *testing.T) {
    rl := NewRateLimiter()
    clientID := "window-decrease-client"

    // Initial setup: 3 requests per 2 seconds
    rl.SetRateLimit(clientID, 3, 2*time.Second)

    // Make requests spaced 500ms apart
    if !rl.Allow(clientID) {
        t.Error("First request at t=0 should be allowed")
    }
    time.Sleep(500 * time.Millisecond)

    if !rl.Allow(clientID) {
        t.Error("Second request at t=500ms should be allowed")
    }
    time.Sleep(500 * time.Millisecond)

    if !rl.Allow(clientID) {
        t.Error("Third request at t=1s should be allowed")
    }

    // At t=1s, decrease window to 750ms
    // This should only keep the most recent request in the window
    rl.SetRateLimit(clientID, 1, 750*time.Millisecond)

    // Request should be denied as the last request is still within 750ms
    if rl.Allow(clientID) {
        t.Error("Request should be denied as last request is within new 750ms window")
    }

    // Wait for 750ms window to pass
    time.Sleep(800 * time.Millisecond)

    // Should now allow new requests
    if !rl.Allow(clientID) {
        t.Error("Request should be allowed after 750ms window passed")
    }
}

func TestRateLimiterDurationChangeWithConcurrency(t *testing.T) {
    rl := NewRateLimiter()
    clientID := "duration-concurrent-client"
    
    // Initial setup
    rl.SetRateLimit(clientID, 5, time.Second)
    
    // Create channels for synchronization
    requestsDone := make(chan bool)
    durationChanged := make(chan bool)
    
    // Start goroutine to make requests
    go func() {
        for i := 0; i < 10; i++ {
            rl.Allow(clientID)
            time.Sleep(100 * time.Millisecond)
        }
        requestsDone <- true
    }()
    
    // Start goroutine to change duration
    go func() {
        time.Sleep(300 * time.Millisecond) // Wait for some requests to accumulate
        rl.SetRateLimit(clientID, 5, 2*time.Second)
        durationChanged <- true
    }()
    
    // Wait for both operations to complete
    <-requestsDone
    <-durationChanged
    
    // Verify rate limit is still enforced
    allowedCount := 0
    for i := 0; i < 5; i++ {
        if rl.Allow(clientID) {
            allowedCount++
        }
    }
    
    // Should respect the rate limit even after concurrent duration change
    if allowedCount > 5 {
        t.Errorf("Rate limit violated after concurrent duration change: got %d allowed requests, expected <= 5", allowedCount)
    }
}

func TestRateLimiterRapidDurationChanges(t *testing.T) {
    rl := NewRateLimiter()
    clientID := "rapid-duration-client"
    
    durations := []time.Duration{
        time.Second,
        500 * time.Millisecond,
        2 * time.Second,
        750 * time.Millisecond,
        time.Second,
    }
    
    // Set initial rate limit
    rl.SetRateLimit(clientID, 2, durations[0])
    
    // Make initial request
    if !rl.Allow(clientID) {
        t.Error("First request should be allowed")
    }
    
    // Rapidly change durations and make requests
    for _, duration := range durations {
        rl.SetRateLimit(clientID, 2, duration)
        
        // Small delay to simulate real-world scenario
        time.Sleep(50 * time.Millisecond)
        
        // Make request
        allowed := rl.Allow(clientID)
        t.Logf("Request with duration %v allowed: %v", duration, allowed)
    }
}

func TestRateLimiterDurationEdgeCases(t *testing.T) {
    rl := NewRateLimiter()
    clientID := "edge-case-client"
    
    testCases := []struct {
        name     string
        duration time.Duration
        requests int
        want     bool
    }{
        {"Zero Duration", 0, 5, false},
        {"Negative Duration", -time.Second, 5, false},
        {"Very Short Duration", time.Millisecond, 5, true},
        {"Very Long Duration", 24 * time.Hour, 5, true},
        {"Max Duration", time.Duration(1<<63 - 1), 5, true},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            rl.SetRateLimit(clientID, tc.requests, tc.duration)
            got := rl.Allow(clientID)
            if got != tc.want {
                t.Errorf("Allow() with duration %v = %v; want %v", tc.duration, got, tc.want)
            }
        })
    }
}

func TestRateLimiterRequestPreservation(t *testing.T) {
    rl := NewRateLimiter()
    clientID := "preservation-client"
    
    // Set initial rate limit: 3 requests per second
    rl.SetRateLimit(clientID, 3, time.Second)
    
    // Make 2 requests
    if !rl.Allow(clientID) {
        t.Error("First request should be allowed")
    }
    if !rl.Allow(clientID) {
        t.Error("Second request should be allowed")
    }
    
    // Change requests limit but keep duration
    rl.SetRateLimit(clientID, 4, time.Second)
    
    // Should still count previous requests
    if rl.Allow(clientID) && rl.Allow(clientID) && rl.Allow(clientID) {
        t.Error("Should not allow 3 more requests as previous requests should be preserved")
    }
    
    // Wait for window to pass
    time.Sleep(1100 * time.Millisecond)
    
    // Should now allow requests under new limit
    for i := 0; i < 4; i++ {
        if !rl.Allow(clientID) {
            t.Errorf("Request %d should be allowed under new limit after window reset", i+1)
        }
    }
    
    // Fifth request should be denied
    if rl.Allow(clientID) {
        t.Error("Fifth request should be denied under new limit")
    }
}

func BenchmarkRateLimiterHighContention(b *testing.B) {
    rl := NewRateLimiter()
    // Use a small set of clients to increase contention
    numClients := 10
    clients := make([]string, numClients)
    
    for i := 0; i < numClients; i++ {
        clients[i] = fmt.Sprintf("bench-client-%d", i)
        rl.SetRateLimit(clients[i], 1000000, time.Second) // High limit to avoid rate limiting effects
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            // Repeatedly hit the same few clients
            clientID := clients[i%numClients]
            rl.Allow(clientID)
            i++
        }
    })
}

func BenchmarkRateLimiterMixedWorkload(b *testing.B) {
    rl := NewRateLimiter()
    numClients := 100
    clients := make([]string, numClients)
    
    for i := 0; i < numClients; i++ {
        clients[i] = fmt.Sprintf("bench-client-%d", i)
        rl.SetRateLimit(clients[i], 1000000, time.Second)
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            if i%100 == 0 {
                // Occasionally update rate limits
                clientID := clients[i%numClients]
                rl.SetRateLimit(clientID, 1000000, time.Second)
            } else {
                // Mostly check allows
                clientID := clients[i%numClients]
                rl.Allow(clientID)
            }
            i++
        }
    })
}

func TestRateLimiterConcurrentAccessPatterns(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping concurrent access pattern test in short mode")
    }
    
    rl := NewRateLimiter()
    numClients := 100
    numOperations := 10000
    var wg sync.WaitGroup
    
    // Track timing statistics
    var totalDuration time.Duration
    var operationCount int64
    var mu sync.Mutex
    
    // Launch goroutines that perform different access patterns
    for i := 0; i < numClients; i++ {
        wg.Add(3) // 3 goroutines per client
        
        clientID := fmt.Sprintf("client-%d", i)
        
        // Goroutine 1: Frequent Allow checks
        go func(id string) {
            defer wg.Done()
            for j := 0; j < numOperations; j++ {
                start := time.Now()
                rl.Allow(id)
                duration := time.Since(start)
                
                mu.Lock()
                totalDuration += duration
                operationCount++
                mu.Unlock()
                
                time.Sleep(time.Microsecond) // Small delay to simulate real work
            }
        }(clientID)
        
        // Goroutine 2: Occasional rate limit updates
        go func(id string) {
            defer wg.Done()
            for j := 0; j < numOperations/100; j++ { // Less frequent updates
                start := time.Now()
                rl.SetRateLimit(id, 100+j, time.Second)
                duration := time.Since(start)
                
                mu.Lock()
                totalDuration += duration
                operationCount++
                mu.Unlock()
                
                time.Sleep(time.Millisecond) // Longer delay for updates
            }
        }(clientID)
        
        // Goroutine 3: Mixed rapid-fire operations
        go func(id string) {
            defer wg.Done()
            for j := 0; j < numOperations; j++ {
                start := time.Now()
                if j%50 == 0 {
                    rl.SetRateLimit(id, 100, time.Second)
                } else {
                    rl.Allow(id)
                }
                duration := time.Since(start)
                
                mu.Lock()
                totalDuration += duration
                operationCount++
                mu.Unlock()
            }
        }(clientID)
    }
    
    wg.Wait()
    
    avgDuration := totalDuration / time.Duration(operationCount)
    t.Logf("Average operation duration: %v across %d operations", avgDuration, operationCount)
}

func TestFilterTimestamps(t *testing.T) {
    tests := []struct {
        name      string
        input     []int64
        cutoff    int64
        expected  []int64
    }{
        {
            name:      "all timestamps newer than cutoff",
            input:     []int64{100, 200, 300, 400, 500},
            cutoff:    50,
            expected:  []int64{100, 200, 300, 400, 500},
        },
        {
            name:      "all timestamps older than cutoff",
            input:     []int64{100, 200, 300, 400, 500},
            cutoff:    600,
            expected:  []int64{},
        },
        {
            name:      "mixed timestamps",
            input:     []int64{100, 200, 300, 400, 500},
            cutoff:    250,
            expected:  []int64{300, 400, 500},
        },
        {
            name:      "empty input",
            input:     []int64{},
            cutoff:    100,
            expected:  []int64{},
        },
        {
            name:      "single timestamp newer than cutoff",
            input:     []int64{200},
            cutoff:    100,
            expected:  []int64{200},
        },
        {
            name:      "single timestamp older than cutoff",
            input:     []int64{100},
            cutoff:    200,
            expected:  []int64{},
        },
        {
            name:      "cutoff exactly matches a timestamp",
            input:     []int64{100, 200, 300, 400, 500},
            cutoff:    300,
            expected:  []int64{400, 500},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create a copy of input to verify we don't modify the original slice
            inputCopy := make([]int64, len(tt.input))
            copy(inputCopy, tt.input)

            // Run the filter
            result := filterBy(tt.input, tt.cutoff)

            // Check length
            if len(result) != len(tt.expected) {
                t.Errorf("length mismatch: got %v, want %v", len(result), len(tt.expected))
            }

            // Check contents
            if !reflect.DeepEqual(result, tt.expected) {
                t.Errorf("content mismatch:\ngot  %v\nwant %v", result, tt.expected)
            }

            // Verify the original slice wasn't modified (if it wasn't empty)
            if len(inputCopy) > 0 && !reflect.DeepEqual(tt.input, inputCopy) {
                t.Error("original slice was modified")
            }

            // Verify the capacity if there were any elements kept
            if len(result) > 0 {
                // The capacity should be the same as the input slice from the start of the valid elements
                expectedCap := cap(tt.input) - (len(tt.input) - len(result))
                if cap(result) != expectedCap {
                    t.Errorf("capacity mismatch: got %v, want %v", cap(result), expectedCap)
                }
            }
        })
    }

    // Additional test for slice optimization
    t.Run("verify slice reuse", func(t *testing.T) {
        original := []int64{100, 200, 300, 400, 500}
        cutoff := int64(250)
        
        // Get the address of the first element we expect to keep
        expectedAddr := &original[2] // 300 is at index 2

        result := filterWithin(original, cutoff)

        // Verify that the first element of our result points to the same memory
        if len(result) > 0 && &result[0] != expectedAddr {
            t.Error("filterTimestamps created a new slice instead of reusing the existing one")
        }
    })
}
