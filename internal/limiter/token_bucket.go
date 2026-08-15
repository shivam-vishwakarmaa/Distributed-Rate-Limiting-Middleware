package limiter

import (
	"context"
	"fmt"
	"time"

	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/token_bucket.lua
var tokenBucketScript string

type TokenBucketLimiter struct {
	client  *redis.Client
	script  *redis.Script
	nowFunc func() time.Time
}

func NewTokenBucketLimiter(client *redis.Client) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		client:  client,
		script:  redis.NewScript(tokenBucketScript),
		nowFunc: time.Now,
	}
}

func (l *TokenBucketLimiter) SetNowFunc(f func() time.Time) {
	l.nowFunc = f
}

func (l *TokenBucketLimiter) Allow(ctx context.Context, req CheckRequest) (*Result, error) {
	key := fmt.Sprintf("ratelimit:token_bucket:%s:%s", req.RouteID, req.ClientID)

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
	tokensInt, _ := resSlice[1].(int64)

	allowed := allowedInt == 1
	remaining := int(tokensInt)

	return &Result{
		Allowed:   allowed,
		Remaining: remaining,
	}, nil
}
