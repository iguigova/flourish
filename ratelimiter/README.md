
# Rate Limiter

## Notes
        
- A _naive_ implementation would be to reject requests arriving faster than (duration / requests). It would be sufficient to only compare against the timestamp of the last allowed request stored as atomic.Int64 but it would not fully satisfy the requirements. 

- A _functional_ implementation would also be simple but would not follow common Go patterns or allow for a distributed version. Here is an example: 

``` clojure
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

``` go
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
  - State could be keyed by a client id (or struct), and persist a slice of request timestamps (filtered by the duration window)
  - Maps are not thread-safe.

- A _distributed_ implementation would handle its own state, would add overhead but provide scalability in addressing the need for high throughput, low latency requirement
        
### Thread Safety Notes

"Don't communicate by sharing memory; share memory by communicating." --Rob Pike

Thread-safety considerations include atomic vs sync.Map vs mutexes vs channels         
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
  



