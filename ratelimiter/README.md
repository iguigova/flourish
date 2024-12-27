# Rate Limiter
        
## Usage

### Prerequisites
- Go 1.20 or higher

### Installation

```bash
go get github.com/iguigova/flourish/ratelimiter
```

### RateLimiter Interface:

- `Allow(clientID string) bool`: Checks if a request should be allowed
- `SetRateLimit(clientID string, requests int, duration time.Duration)`: Configures client-specific limits

### Example

See the example usage in [main.go](../main.go)
        
## Implemantation Details

### Core implementation

- InMemoryRateLimiter:
   - Thread-safe implementation using mutex synchronization
   - Sliding window algorithm for request tracking
   - Memory-efficient state management
   - Use int64 rather than time.Time to improve memory uasage and performance
   - Use of limiter-level locks and client-level locks to minimize the duration of required locking
        
### Time Breakdown

- Core Implementation: 2 hours
- Testing: 2 hours
- Documentation: 1 hour
- Updates: 2 hours
        
### Considerations
- A _naive_ implementation would be to reject requests arriving faster than (duration / requests). It would be sufficient to only compare against the timestamp of the last allowed request stored as atomic.Int64 but it would not fully satisfy the requirements. 

- A _functional_ implementation would also be simple but would not follow common Go patterns or allow for a distributed version. Here is an example: 

```clojure
(defn get-times [state request cutoff]
  (->> (get @state request [])
       (filterv #(> % cutoff))))

(defn update-times [state request time times]
  (swap! state assoc request (conj times time)))

# state is an atom and thread safe. it is enclosed and a hdden implementation detail. not a swapable implementation detail
(defn rate-limiter [limit window-ms]
  (let [state (atom {})]
    (fn [request]
      (let [now (System/currentTimeMillis)
            cutoff (- now window-ms)
            times (get-times state request cutoff)]
        (update-times state request now times)
        (if (< (count times) limit) 200 400)))))

# usage
(def a (rate-limiter 1 10000))
(def b (rate-limiter 100 1000))
``` 

```go
package ratelimiter

import (
	"sync"
	"time"
)

// RateLimiter is a function that checks if a request should be allowed
type RateLimiter func(clientID string) bool

// NewRateLimiter creates a new rate limiter with specified requests and duration window
func NewRateLimiter(requests int, duration time.Duration) RateLimiter {
	state := make(map[string][]time.Time)
	var mu sync.RWMutex

	return func(clientID string) bool {
		now := time.Now()
		cutoff := now.Add(-duration)
		
		// Get current timestamps and update in a single critical section
		mu.Lock()
		timestamps, exists := state[clientID]
		if !exists {
			timestamps = []time.Time{}
		}
		
		// Filter and update in one pass
		validTimestamps := make([]time.Time, 0, len(timestamps)+1)
		for _, t := range timestamps {
			if t.After(cutoff) {
				validTimestamps = append(validTimestamps, t)
			}
		}
		validTimestamps = append(validTimestamps, now)
		state[clientID] = validTimestamps
		mu.Unlock()
		
		return len(validTimestamps)-1 < requests // -1 because we already added the new time
	}
}

```
    
- A _local_ implementation could keep its state in a hash table / go's maps
  - State could be keyed by a client id (or struct), and could persist a slice of request timestamps (filtered by the duration window)
  - Maps are not thread-safe.

- A _distributed_ implementation would handle its own state, would add overhead but provide scalability in addressing the need for high throughput, low latency requirement
        
#### Notes on thread-safety

"Don't communicate by sharing memory; share memory by communicating." --Rob Pike

Thread-safety considerations include `atomic` vs `sync.Map` vs `mutexes` vs `channels`         
  - Use sync.Map when:
    - In high throughput scenarios
    - Entries are written once and read many times
    - Different goroutines access different keys
    - There's high read contention
  - Use Mutex when:
    - Protecting shared state that needs quick access
    - Managing simple shared resources
    - Need fine-grained locking
    - Performance is critical (mutexes are generally faster)
    - The shared state is accessed frequently but modified rarely
  - Use Channels when:
    - Transferring ownership of data
    - Coordinating tasks between goroutines
    - Signaling events between parts of the program
    - Implementing pipelines or worker pools
    - Broadcasting to multiple goroutines

### Future Improvements

1. Distributed backend support:
   - Redis integration
   - Consistent hashing
   - Cluster coordination

2. Enhanced monitoring:
   - Request tracking
   - Alert configuration
        
## Test Coverage (by Claude.ai)

Basic Rate Limiting Functionality ✓ Fully Covered:

- TestRateLimiterBasicFunctionality: Tests basic allow/deny logic
- TestRateLimiterWindowSliding: Tests sliding window algorithm
- TestRateLimiterMultipleClients: Tests different limits for different clients
- TestRateLimiterDefaultLimits: Tests default rate limit behavior

Thread Safety & Concurrency ✓ Fully Covered:

- TestRateLimiterConcurrency: Tests concurrent request handling
- TestRateLimiterConcurrentModification: Tests concurrent modifications
- TestRateLimiterStress: High-load concurrent testing
- TestRateLimiterMaxClients: Tests concurrent client creation

Performance & Memory Efficiency ✓ Fully Covered:

- BenchmarkRateLimiter: Basic performance benchmarking
- BenchmarkRateLimiterConcurrent: Concurrent performance testing
- BenchmarkRateLimiterMemoryAllocation: Memory allocation testing
- TestRateLimiterUnderMemoryPressure: Memory pressure handling
- TestRateLimiterGracefulDegradation: Performance degradation testing

Extensibility & Distributed Systems ✓ Fully Covered:

- TestRateLimiterDistributedBackendCompatibility: Tests interface compatibility
- Mock implementations showing extensibility
- MetricsRateLimiter implementation showing monitoring capabilities

Edge Cases & Error Handling ✓ Fully Covered:

- TestRateLimiterInvalidDuration: Tests invalid time windows
- TestRateLimiterOverflow: Tests numeric overflow scenarios
- TestRateLimiterZeroLimit: Tests zero limit case
- TestRateLimiterNegativeLimit: Tests negative limit case
- TestRateLimiterTimeJump: Tests time-related edge cases
- TestRateLimiterRecoveryAfterPanic: Tests system recovery

Long-term Stability ✓ Fully Covered:

- TestRateLimiterLongRunningStability: Tests sustained operation
- TestRateLimiterTimeSimulation: Tests behavior over simulated time periods
- TestRateLimiterBurstyTraffic: Tests handling of traffic spikes

### Additional Strengths:

- Clean-up and maintenance tests
- Memory pressure handling
- Comprehensive benchmarking suite
- Distributed systems compatibility

### Potential Improvements:

- Could add more tests for:
  - Different time window configurations (currently focused on seconds/milliseconds)
  - Rate limit adjustments during high load
  - Specific distributed system scenarios
- Could expand metrics testing to cover more detailed statistics

### Summary
The test suite is very comprehensive and thoroughly covers all the core requirements from the problem statement. It includes both unit tests and integration tests, covers edge cases, performance scenarios, and even includes future extensibility considerations. The tests are well-structured and follow good testing practices with clear naming and separation of concerns.

#### Running Tests

```bash
go test ./... -v        # Run all tests with verbose output
go test -bench=.       # Run benchmarks
go test -race         # Run tests with race detector
```

#### Benchmark results
- [2 level locks](https://github.com/iguigova/flourish/commit/3f0856117d3214addcb039cba64afeba3db59710)
```        
go test -bench=.
goos: linux
goarch: amd64
pkg: github.com/iguigova/flourish/ratelimiter
cpu: 11th Gen Intel(R) Core(TM) i7-11850H @ 2.50GHz
BenchmarkRateLimiter-16                    	 2051318	       576.3 ns/op
BenchmarkRateLimiterConcurrent-16          	 2382546	       501.8 ns/op
BenchmarkRateLimiterMultipleClients-16     	17158329	        60.68 ns/op
BenchmarkRateLimiterMemoryAllocation-16    	 2038870	       586.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkRateLimiterHighContention-16      	 1000000	      3134 ns/op
BenchmarkRateLimiterMixedWorkload-16       	 3060484	       717.3 ns/op
PASS
ok  	github.com/iguigova/flourish/ratelimiter	20.335s
```

- [filterTimestamp shortcut loop](https://github.com/iguigova/flourish/commit/6fef615df5685657d073ebeb0e9bc056f8eee955)
```
go test -bench=.
goos: linux
goarch: amd64
pkg: github.com/iguigova/flourish/ratelimiter
cpu: 11th Gen Intel(R) Core(TM) i7-11850H @ 2.50GHz
BenchmarkRateLimiter-16                    	25025702	        45.72 ns/op
BenchmarkRateLimiterConcurrent-16          	12385672	        97.87 ns/op
BenchmarkRateLimiterMultipleClients-16     	24017977	        50.01 ns/op
BenchmarkRateLimiterMemoryAllocation-16    	25609137	        46.62 ns/op	       0 B/op	       0 allocs/op
BenchmarkRateLimiterHighContention-16      	12232365	        87.81 ns/op
BenchmarkRateLimiterMixedWorkload-16       	20413990	        58.29 ns/op
PASS
ok  	github.com/iguigova/flourish/ratelimiter	17.530s
```