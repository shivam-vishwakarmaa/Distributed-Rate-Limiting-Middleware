package limiter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/sliding_window_log.lua
var slidingWindowLogScript string

type SlidingWindowLogLimiter struct {
	client  *redis.Client
	script  *redis.Script
	nowFunc func() time.Time
}

func NewSlidingWindowLogLimiter(client *redis.Client) *SlidingWindowLogLimiter {
	return &SlidingWindowLogLimiter{
		client:  client,
		script:  redis.NewScript(slidingWindowLogScript),
		nowFunc: time.Now,
	}
}

func (l *SlidingWindowLogLimiter) SetNowFunc(f func() time.Time) {
	l.nowFunc = f
}

func generateSuffix() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (l *SlidingWindowLogLimiter) Allow(ctx context.Context, req CheckRequest) (*Result, error) {
	key := fmt.Sprintf("ratelimit:sliding_window_log:%s:%s", req.RouteID, req.ClientID)

	now := l.nowFunc()
	nowMs := now.UnixMilli()
	windowMs := int64(req.WindowSeconds) * 1000

	memberSuffix := generateSuffix()

	res, err := l.script.Run(ctx, l.client, []string{key}, nowMs, windowMs, req.Limit, memberSuffix).Result()
	if err != nil {
		return nil, err
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) != 2 {
		return nil, fmt.Errorf("unexpected script result format")
	}

	allowedInt, _ := resSlice[0].(int64)
	countInt, _ := resSlice[1].(int64)

	allowed := allowedInt == 1
	remaining := req.Limit - int(countInt)
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:   allowed,
		Remaining: remaining,
	}, nil
}
