//go:build integration

package integration

import (
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

	for i := 0; i < 1100; i++ {
		req, _ := http.NewRequest("GET", middlewareURL+"/", nil)
		req.Header.Set("X-API-Key", "test-user-basic")
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
	client := &http.Client{Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	
	numGoroutines := 1050
	wg.Add(numGoroutines)

	var allowedCount int32
	var deniedCount int32

	startCh := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			<-startCh
			
			req, _ := http.NewRequest("GET", middlewareURL+"/", nil)
			req.Header.Set("X-API-Key", "test-user-race")
			
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				atomic.AddInt32(&allowedCount, 1)
			} else if resp.StatusCode == http.StatusTooManyRequests {
				atomic.AddInt32(&deniedCount, 1)
			}
		}()
	}

	close(startCh)
	wg.Wait()

	assert.Equal(t, int32(1000), allowedCount)
	assert.Equal(t, int32(50), deniedCount)
}
