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

func TestTokenBucket_Basic(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewTokenBucketLimiter(client)
	ctx := context.Background()
	
	currentTime := time.Now()
	l.SetNowFunc(func() time.Time {
		return currentTime
	})

	req := limiter.CheckRequest{
		RouteID:         "/api/tb_basic",
		ClientID:        "user",
		Capacity:        2,
		RefillPerSecond: 1,
	}

	// Burst 2
	res, _ := l.Allow(ctx, req)
	assert.True(t, res.Allowed)
	assert.Equal(t, 1, res.Remaining)

	res, _ = l.Allow(ctx, req)
	assert.True(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)

	// Denied
	res, _ = l.Allow(ctx, req)
	assert.False(t, res.Allowed)

	// Wait 1 second (1 token refills)
	currentTime = currentTime.Add(1 * time.Second)
	res, _ = l.Allow(ctx, req)
	assert.True(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)
	
	// Fast follow-up should be denied
	res, _ = l.Allow(ctx, req)
	assert.False(t, res.Allowed)
}

func TestTokenBucket_ConcurrentRace(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewTokenBucketLimiter(client)
	ctx := context.Background()
	req := limiter.CheckRequest{
		RouteID:         "/api/tb_race",
		ClientID:        "user-race",
		Capacity:        10,
		RefillPerSecond: 1, // won't refill fast enough during test
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

func TestTokenBucket_BurstBehavior(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewTokenBucketLimiter(client)
	
	currentTime := time.Now()
	l.SetNowFunc(func() time.Time {
		return currentTime
	})

	ctx := context.Background()
	req := limiter.CheckRequest{
		RouteID:         "/api/burst",
		ClientID:        "user-burst",
		Capacity:        10,
		RefillPerSecond: 2,
	}

	// T=0: idle bucket allows a burst up to capacity (10)
	for i := 0; i < 10; i++ {
		res, _ := l.Allow(ctx, req)
		assert.True(t, res.Allowed)
	}

	// 11th should be denied
	res, _ := l.Allow(ctx, req)
	assert.False(t, res.Allowed)

	// Wait 2.5 seconds. Refill rate is 2 per second, so we should get 5 tokens.
	currentTime = currentTime.Add(2500 * time.Millisecond)

	// Should allow 5
	for i := 0; i < 5; i++ {
		res, _ = l.Allow(ctx, req)
		assert.True(t, res.Allowed)
	}

	// 6th should be denied
	res, _ = l.Allow(ctx, req)
	assert.False(t, res.Allowed)
}
