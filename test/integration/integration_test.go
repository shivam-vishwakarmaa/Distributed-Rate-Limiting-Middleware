//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const middlewareURL = "http://localhost:8080"

func TestIntegration_BasicRateLimit(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	var allowed, denied int
	apiKey := fmt.Sprintf("test-user-basic-%d", time.Now().UnixNano())

	for i := 0; i < 1100; i++ {
		req, _ := http.NewRequest("POST", middlewareURL+"/", nil)
		req.Header.Set("X-API-Key", apiKey)
		resp, err := client.Do(req)
		require.NoError(t, err)

		if resp.StatusCode == http.StatusOK {
			allowed++
		} else if resp.StatusCode == http.StatusTooManyRequests {
			denied++
		}
		resp.Body.Close()
	}

	assert.Equal(t, 1000, allowed)
	assert.Equal(t, 100, denied)
}

func TestIntegration_ConcurrentRace(t *testing.T) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConns = 2000
	tr.MaxConnsPerHost = 2000
	tr.MaxIdleConnsPerHost = 2000
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}
	var wg sync.WaitGroup
	
	numGoroutines := 1050
	wg.Add(numGoroutines)

	var allowedCount int32
	var deniedCount int32
	apiKey := fmt.Sprintf("test-user-race-%d", time.Now().UnixNano())

	startCh := make(chan struct{})
	semaphore := make(chan struct{}, 100)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			<-startCh
			
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			req, _ := http.NewRequest("POST", middlewareURL+"/", nil)
			req.Header.Set("X-API-Key", apiKey)
			
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Request error:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				atomic.AddInt32(&allowedCount, 1)
			} else if resp.StatusCode == http.StatusTooManyRequests {
				atomic.AddInt32(&deniedCount, 1)
			}
		}(i)
	}

	close(startCh)
	wg.Wait()

	assert.Equal(t, int32(1000), allowedCount)
	assert.Equal(t, int32(50), deniedCount)
}
