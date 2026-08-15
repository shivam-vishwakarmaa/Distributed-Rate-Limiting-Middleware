package unit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rate-limiter/internal/limiter"
)

func TestSlidingWindowLog_Basic(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewSlidingWindowLogLimiter(client)
	ctx := context.Background()

	req := limiter.CheckRequest{
		RouteID:       "/api/sw_basic",
		ClientID:      "user",
		Limit:         2,
		WindowSeconds: 60,
	}

	res, _ := l.Allow(ctx, req)
	assert.True(t, res.Allowed)
	assert.Equal(t, 1, res.Remaining)

	res, _ = l.Allow(ctx, req)
	assert.True(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)

	res, _ = l.Allow(ctx, req)
	assert.False(t, res.Allowed)
}

func TestSlidingWindowLog_ConcurrentRace(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewSlidingWindowLogLimiter(client)
	ctx := context.Background()
	req := limiter.CheckRequest{
		RouteID:       "/api/sw_race",
		ClientID:      "user-race",
		Limit:         10,
		WindowSeconds: 60,
	}

	var allowedCount int32
	var wg sync.WaitGroup
	numGoroutines := 50
	wg.Add(numGoroutines)

	startCh := make(chan struct{})
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			<-startCh
			res, err := l.Allow(ctx, req)
			if err == nil && res.Allowed {
				atomic.AddInt32(&allowedCount, 1)
			}
		}()
	}
	close(startCh)
	wg.Wait()

	assert.Equal(t, int32(10), allowedCount)
}

func TestSlidingWindowLog_BoundarySpike(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewSlidingWindowLogLimiter(client)
	
	currentTime := time.Now()
	l.SetNowFunc(func() time.Time {
		return currentTime
	})

	ctx := context.Background()
	req := limiter.CheckRequest{
		RouteID:       "/api/boundary",
		ClientID:      "user-boundary",
		Limit:         5,
		WindowSeconds: 60,
	}

	// T = 59.999s (simulate requests just before the minute boundary)
	currentTime = currentTime.Add(59999 * time.Millisecond)
	for i := 0; i < 5; i++ {
		res, _ := l.Allow(ctx, req)
		assert.True(t, res.Allowed)
	}

	// T = 60.001s (simulate requests just after the minute boundary)
	currentTime = currentTime.Add(2 * time.Millisecond)
	
	// A fixed window resetting on the minute would allow these because it's a new minute.
	// But sliding window log should DENY them, because they are within the same 60s sliding window 
	// as the previous 5 requests (59.999 to 60.001 is only a 2ms delta).
	for i := 0; i < 5; i++ {
		res, _ := l.Allow(ctx, req)
		assert.False(t, res.Allowed)
	}

	// T = 120.000s (60 seconds after the first batch)
	currentTime = currentTime.Add(60000 * time.Millisecond)
	for i := 0; i < 5; i++ {
		res, _ := l.Allow(ctx, req)
		assert.True(t, res.Allowed)
	}
}
