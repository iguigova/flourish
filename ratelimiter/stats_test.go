package ratelimiter

import (
	"sync"
	"testing"
	"time"
)

func TestStatsKeeperBasicFunctionality(t *testing.T) {
	sk := NewStatsKeeper()
	clientID := "test-client"

	// Test initial state
	stats, exists := sk.GetClientStats(clientID)
	if exists {
		t.Error("Expected no stats for new client")
	}

	// Test recording allows
	sk.RecordAllow(clientID)
	sk.RecordAllow(clientID)

	// Test recording denies
	sk.RecordDeny(clientID)

	// Verify counts
	stats, exists = sk.GetClientStats(clientID)
	if !exists {
		t.Error("Expected stats to exist after recording")
	}
	if stats.Allowed != 2 {
		t.Errorf("Expected 2 allows, got %d", stats.Allowed)
	}
	if stats.Denied != 1 {
		t.Errorf("Expected 1 deny, got %d", stats.Denied)
	}
	if stats.LastAccess.IsZero() {
		t.Error("Expected LastAccess to be set")
	}
}

func TestStatsKeeperGlobalStats(t *testing.T) {
	sk := NewStatsKeeper()
	
	// Record stats for multiple clients
	sk.RecordAllow("client1")
	sk.RecordAllow("client1")
	sk.RecordDeny("client1")
	
	sk.RecordAllow("client2")
	sk.RecordDeny("client2")
	sk.RecordDeny("client2")
	
	// Check global stats
	global := sk.GetGlobalStats()
	if global.Allowed != 3 {
		t.Errorf("Expected 3 total allows, got %d", global.Allowed)
	}
	if global.Denied != 3 {
		t.Errorf("Expected 3 total denies, got %d", global.Denied)
	}
	if global.LastAccess.IsZero() {
		t.Error("Expected LastAccess to be set in global stats")
	}
}

func TestStatsKeeperCleanup(t *testing.T) {
	sk := NewStatsKeeper()
	
	// Record some stats
	sk.RecordAllow("client1")
	time.Sleep(200 * time.Millisecond) // Increased sleep time
	sk.RecordAllow("client2")
	
	// Sleep additional time to ensure proper aging
	time.Sleep(100 * time.Millisecond)
	
	// Clean up entries older than 200ms
	sk.Cleanup(200 * time.Millisecond)
	
	// client1 should be cleaned up, client2 should remain
	_, exists1 := sk.GetClientStats("client1")
	_, exists2 := sk.GetClientStats("client2")
	
	if exists1 {
		t.Error("Expected client1 stats to be cleaned up")
	}
	if !exists2 {
		t.Error("Expected client2 stats to remain")
	}
}

func TestStatsKeeperReset(t *testing.T) {
	sk := NewStatsKeeper()
	
	// Record some stats
	sk.RecordAllow("client1")
	sk.RecordDeny("client2")
	
	// Reset stats
	sk.Reset()
	
	// Verify all stats are cleared
	_, exists1 := sk.GetClientStats("client1")
	_, exists2 := sk.GetClientStats("client2")
	global := sk.GetGlobalStats()
	
	if exists1 || exists2 {
		t.Error("Expected all client stats to be cleared after reset")
	}
	if global.Allowed != 0 || global.Denied != 0 {
		t.Error("Expected global stats to be zeroed after reset")
	}
}

func TestStatsKeeperConcurrency(t *testing.T) {
	sk := NewStatsKeeper()
	const numGoroutines = 100
	const numOperations = 1000
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Launch goroutines to concurrently record stats
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			clientID := "concurrent-client"
			
			for j := 0; j < numOperations; j++ {
				if j%2 == 0 {
					sk.RecordAllow(clientID)
				} else {
					sk.RecordDeny(clientID)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify total counts
	stats, _ := sk.GetClientStats("concurrent-client")
	expectedOperations := numGoroutines * numOperations
	actualOperations := int(stats.Allowed + stats.Denied)
	
	if actualOperations != expectedOperations {
		t.Errorf("Expected %d total operations, got %d", expectedOperations, actualOperations)
	}
	if stats.Allowed != stats.Denied {
		t.Errorf("Expected equal allows and denies, got %d allows and %d denies", 
			stats.Allowed, stats.Denied)
	}
}

func TestStatsKeeperLastAccess(t *testing.T) {
	sk := NewStatsKeeper()
	clientID := "timestamp-client"
	
	// Record initial access
	beforeAccess := time.Now()
	time.Sleep(10 * time.Millisecond) // Ensure clear time difference
	sk.RecordAllow(clientID)
	afterAccess := time.Now()
	
	stats, _ := sk.GetClientStats(clientID)
	if stats.LastAccess.Before(beforeAccess) || stats.LastAccess.After(afterAccess) {
		t.Errorf("LastAccess (%v) should be between %v and %v", 
			stats.LastAccess, beforeAccess, afterAccess)
	}
	
	// Wait and record another access
	time.Sleep(100 * time.Millisecond)
	beforeSecondAccess := time.Now()
	sk.RecordDeny(clientID)
	afterSecondAccess := time.Now()
	
	newStats, _ := sk.GetClientStats(clientID)
	if newStats.LastAccess.Before(beforeSecondAccess) || newStats.LastAccess.After(afterSecondAccess) {
		t.Errorf("New LastAccess (%v) should be between %v and %v", 
			newStats.LastAccess, beforeSecondAccess, afterSecondAccess)
	}
	
	if !newStats.LastAccess.After(stats.LastAccess) {
		t.Errorf("New LastAccess (%v) should be after original LastAccess (%v)", 
			newStats.LastAccess, stats.LastAccess)
	}
}

func TestStatsKeeperMultipleClients(t *testing.T) {
	sk := NewStatsKeeper()
	
	// Test with multiple clients having different patterns
	clients := []struct {
		id          string
		allows      int
		denies      int
		sleepAfter  time.Duration
	}{
		{"client1", 5, 2, 0},
		{"client2", 3, 7, 50 * time.Millisecond},
		{"client3", 0, 4, 100 * time.Millisecond},
	}
	
	var expectedAllows, expectedDenies uint64
	
	for _, c := range clients {
		for i := 0; i < c.allows; i++ {
			sk.RecordAllow(c.id)
			expectedAllows++
		}
		for i := 0; i < c.denies; i++ {
			sk.RecordDeny(c.id)
			expectedDenies++
		}
		time.Sleep(c.sleepAfter)
	}
	
	// Verify individual client stats
	for _, c := range clients {
		stats, exists := sk.GetClientStats(c.id)
		if !exists {
			t.Errorf("Expected stats to exist for client %s", c.id)
			continue
		}
		if stats.Allowed != uint64(c.allows) {
			t.Errorf("Client %s: expected %d allows, got %d", 
				c.id, c.allows, stats.Allowed)
		}
		if stats.Denied != uint64(c.denies) {
			t.Errorf("Client %s: expected %d denies, got %d", 
				c.id, c.denies, stats.Denied)
		}
	}
	
	// Verify global stats
	global := sk.GetGlobalStats()
	if global.Allowed != expectedAllows {
		t.Errorf("Expected %d total allows, got %d", expectedAllows, global.Allowed)
	}
	if global.Denied != expectedDenies {
		t.Errorf("Expected %d total denies, got %d", expectedDenies, global.Denied)
	}
}

func BenchmarkStatsKeeperRecordAllow(b *testing.B) {
	sk := NewStatsKeeper()
	clientID := "bench-client"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sk.RecordAllow(clientID)
	}
}

func BenchmarkStatsKeeperRecordAllowParallel(b *testing.B) {
	sk := NewStatsKeeper()
	clientID := "bench-client"
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sk.RecordAllow(clientID)
		}
	})
}

func BenchmarkStatsKeeperGetClientStats(b *testing.B) {
	sk := NewStatsKeeper()
	clientID := "bench-client"
	sk.RecordAllow(clientID)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sk.GetClientStats(clientID)
	}
}

func BenchmarkStatsKeeperGetGlobalStats(b *testing.B) {
	sk := NewStatsKeeper()
	
	// Setup some data
	for i := 0; i < 100; i++ {
		clientID := "bench-client"
		sk.RecordAllow(clientID)
		sk.RecordDeny(clientID)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sk.GetGlobalStats()
	}
}
