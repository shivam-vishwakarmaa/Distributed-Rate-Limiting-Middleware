package unit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rate-limiter/internal/limiter"
)

func TestFixedWindow_Basic(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewFixedWindowLimiter(client)
	ctx := context.Background()

	req := limiter.CheckRequest{
		RouteID:       "/api/test",
		ClientID:      "user-1",
		Limit:         3,
		WindowSeconds: 60,
	}

	res, err := l.Allow(ctx, req)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 2, res.Remaining)

	res, err = l.Allow(ctx, req)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 1, res.Remaining)

	res, err = l.Allow(ctx, req)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)

	res, err = l.Allow(ctx, req)
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)
}

func TestFixedWindow_ConcurrentRace(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	l := limiter.NewFixedWindowLimiter(client)
	ctx := context.Background()

	req := limiter.CheckRequest{
		RouteID:       "/api/race",
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
			<-startCh // wait for signal
			res, err := l.Allow(ctx, req)
			if err == nil && res.Allowed {
				atomic.AddInt32(&allowedCount, 1)
			}
		}()
	}

	close(startCh) // start all
	wg.Wait()

	assert.Equal(t, int32(10), allowedCount)
}
