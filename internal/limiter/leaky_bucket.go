package limiter

import (
	"context"
	"fmt"
	"time"

	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/leaky_bucket.lua
var leakyBucketScript string

type LeakyBucketLimiter struct {
	client  *redis.Client
	script  *redis.Script
	nowFunc func() time.Time
}

func NewLeakyBucketLimiter(client *redis.Client) *LeakyBucketLimiter {
	return &LeakyBucketLimiter{
		client:  client,
		script:  redis.NewScript(leakyBucketScript),
		nowFunc: time.Now,
	}
}

func (l *LeakyBucketLimiter) SetNowFunc(f func() time.Time) {
	l.nowFunc = f
}

func (l *LeakyBucketLimiter) Allow(ctx context.Context, req CheckRequest) (*Result, error) {
	key := fmt.Sprintf("ratelimit:leaky_bucket:%s:%s", req.RouteID, req.ClientID)

	now := l.nowFunc()
	nowMs := now.UnixMilli()

	res, err := l.script.Run(ctx, l.client, []string{key}, nowMs, req.Capacity, req.RefillPerSecond).Result()
	if err != nil {
		return nil, err
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) != 2 {
		return nil, fmt.Errorf("unexpected script result format")
	}

	allowedInt, _ := resSlice[0].(int64)
	remainingInt, _ := resSlice[1].(int64)

	allowed := allowedInt == 1
	remaining := int(remainingInt)

	return &Result{
		Allowed:   allowed,
		Remaining: remaining,
	}, nil
}
