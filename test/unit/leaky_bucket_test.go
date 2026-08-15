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

func TestLeakyBucket_Basic(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewLeakyBucketLimiter(client)
	ctx := context.Background()
	
	currentTime := time.Now()
	l.SetNowFunc(func() time.Time {
		return currentTime
	})

	req := limiter.CheckRequest{
		RouteID:         "/api/lb_basic",
		ClientID:        "user",
		Capacity:        2,
		RefillPerSecond: 1, // acts as leak rate
	}

	res, _ := l.Allow(ctx, req)
	assert.True(t, res.Allowed)
	
	res, _ = l.Allow(ctx, req)
	assert.True(t, res.Allowed)
	
	res, _ = l.Allow(ctx, req)
	assert.False(t, res.Allowed)

	currentTime = currentTime.Add(1 * time.Second)
	res, _ = l.Allow(ctx, req)
	assert.True(t, res.Allowed)
	
	res, _ = l.Allow(ctx, req)
	assert.False(t, res.Allowed)
}

func TestLeakyBucket_ConcurrentRace(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewLeakyBucketLimiter(client)
	ctx := context.Background()
	req := limiter.CheckRequest{
		RouteID:         "/api/lb_race",
		ClientID:        "user-race",
		Capacity:        10,
		RefillPerSecond: 1,
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

func TestLeakyBucket_SteadyOutflow(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	lb := limiter.NewLeakyBucketLimiter(client)
	
	currentTime := time.Now()
	lb.SetNowFunc(func() time.Time { return currentTime })

	ctx := context.Background()
	req := limiter.CheckRequest{
		RouteID: "/api/lb_steady", 
		ClientID: "user", 
		Capacity: 5, 
		RefillPerSecond: 1,
	}

	// Initial burst
	for i := 0; i < 5; i++ {
		res, _ := lb.Allow(ctx, req)
		assert.True(t, res.Allowed)
	}

	// 6th is denied
	res, _ := lb.Allow(ctx, req)
	assert.False(t, res.Allowed)

	// In a true leaky bucket with queue, elements leak at 1 per sec.
	// After 1 sec, space opens for 1 item.
	currentTime = currentTime.Add(1 * time.Second)
	res, _ = lb.Allow(ctx, req)
	assert.True(t, res.Allowed)
	
	res, _ = lb.Allow(ctx, req)
	assert.False(t, res.Allowed)
}
