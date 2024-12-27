package ratelimiter

import (
	"sync"
	"sync/atomic"
	"time"
)

// Stats represents a snapshot of statistics at a point in time
type Stats struct {
	Allowed    uint64
	Denied     uint64
	LastAccess time.Time
}

// StatsCollector interface defines the methods for collecting and retrieving statistics
type StatsCollector interface {
	RecordAllow(clientID string)
	RecordDeny(clientID string)
	GetClientStats(clientID string) (Stats, bool)
	GetGlobalStats() Stats
	Cleanup(maxAge time.Duration)
	Reset()
}

// atomicStats holds thread-safe counters for a single client
type atomicStats struct {
	allowed    atomic.Uint64
	denied     atomic.Uint64
	lastAccess atomic.Int64 // UnixNano timestamp
}

// statsKeeper maintains statistics for multiple clients
type statsKeeper struct {
	mu    sync.RWMutex      // Only used for map operations
	stats map[string]*atomicStats
}

// NewStatsKeeper creates a new StatsCollector instance
func NewStatsKeeper() StatsCollector {
	return &statsKeeper{
		stats: make(map[string]*atomicStats),
	}
}

// getStats retrieves or creates stats for a client
func (sk *statsKeeper) getStats(clientID string) *atomicStats {
	sk.mu.RLock()
	stats, exists := sk.stats[clientID]
	sk.mu.RUnlock()
	
	if exists {
		return stats
	}
	
	sk.mu.Lock()
	stats, exists = sk.stats[clientID]
	if !exists {
		stats = &atomicStats{}
		sk.stats[clientID] = stats
	}
	sk.mu.Unlock()
	
	return stats
}

func (sk *statsKeeper) RecordAllow(clientID string) {
	stats := sk.getStats(clientID)
	stats.allowed.Add(1)
	stats.lastAccess.Store(time.Now().UnixNano())
}

func (sk *statsKeeper) RecordDeny(clientID string) {
	stats := sk.getStats(clientID)
	stats.denied.Add(1)
	stats.lastAccess.Store(time.Now().UnixNano())
}

func (sk *statsKeeper) GetClientStats(clientID string) (Stats, bool) {
	sk.mu.RLock()
	stats, exists := sk.stats[clientID]
	sk.mu.RUnlock()
	
	if !exists {
		return Stats{}, false
	}
	
	return Stats{
		Allowed:    stats.allowed.Load(),
		Denied:     stats.denied.Load(),
		LastAccess: time.Unix(0, stats.lastAccess.Load()),
	}, true
}

func (sk *statsKeeper) GetGlobalStats() Stats {
	var total Stats
	now := time.Now()
	
	sk.mu.RLock()
	for _, stats := range sk.stats {
		total.Allowed += stats.allowed.Load()
		total.Denied += stats.denied.Load()
		lastAccess := time.Unix(0, stats.lastAccess.Load())
		if lastAccess.After(total.LastAccess) {
			total.LastAccess = lastAccess
		}
	}
	sk.mu.RUnlock()
	
	if total.LastAccess.IsZero() {
		total.LastAccess = now
	}
	
	return total
}

func (sk *statsKeeper) Cleanup(maxAge time.Duration) {
	threshold := time.Now().Add(-maxAge).UnixNano()
	
	sk.mu.Lock()
	for id, stats := range sk.stats {
		if stats.lastAccess.Load() < threshold {
			delete(sk.stats, id)
		}
	}
	sk.mu.Unlock()
}

func (sk *statsKeeper) Reset() {
	sk.mu.Lock()
	sk.stats = make(map[string]*atomicStats)
	sk.mu.Unlock()
}
