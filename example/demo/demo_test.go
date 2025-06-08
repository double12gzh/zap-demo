package demo

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/double12gzh/zap-demo/router/middleware"
)

func TestConcurrentPing(t *testing.T) {
	// Number of concurrent requests
	numRequests := 10
	var wg sync.WaitGroup

	// Create a channel to collect errors
	errChan := make(chan error, numRequests)

	// Launch concurrent requests
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(requestNum int) {
			defer wg.Done()

			// Create a unique trace ID for each request
			traceID := fmt.Sprintf("test-trace-%d", requestNum)

			// Create a new request
			req, err := http.NewRequest("GET", "http://localhost:8080/ping", nil)
			if err != nil {
				errChan <- fmt.Errorf("failed to create request: %v", err)
				return
			}

			req.Header.Set(middleware.RequestIDHeader, traceID)

			// Create HTTP client with timeout
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			// Send request
			resp, err := client.Do(req)
			if err != nil {
				errChan <- fmt.Errorf("request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			// Check response status
			if resp.StatusCode != http.StatusOK {
				errChan <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				return
			}
		}(i)
	}

	// Wait for all requests to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			t.Errorf("Test failed: %v", err)
		}
	}
}
