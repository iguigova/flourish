package main


import (
	"fmt"
	"sync"
	"time"
	"github.com/iguigova/flourish/ratelimiter"
)

func main() {
	// Create a new rate limiter
	rl := ratelimiter.NewRateLimiter()
	
	// Set rate limits for different clients
	rl.SetRateLimit("client1", 5, time.Second) // 5 requests per second
	rl.SetRateLimit("client2", 10, time.Second) // 10 requests per second
	rl.SetRateLimit("client3", 15, time.Second) // 15 requests per second
	
	// Simulate requests for multiple clients concurrently
	var wg sync.WaitGroup
	clients := []string{"client1", "client2", "client3"}
	
	for _, client := range clients {
		wg.Add(1)
		go func(clientID string) {
			defer wg.Done()
			
			fmt.Printf("Starting requests for %s:\n", clientID)
			for i := 0; i < 20; i++ { // Simulate 20 requests per client
				if rl.Allow(clientID) {
					fmt.Printf("[%s] Request %d: allowed\n", clientID, i+1)
				} else {
					fmt.Printf("[%s] Request %d: denied\n", clientID, i+1)
				}
				time.Sleep(100 * time.Millisecond) // 10 requests per second pace
			}
		}(client)
	}
	
	wg.Wait()
	fmt.Println("All requests completed.")
}
