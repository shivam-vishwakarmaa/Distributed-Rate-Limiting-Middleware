package limiter

import (
	"context"
	"fmt"
	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/fixed_window.lua
var fixedWindowScript string

type FixedWindowLimiter struct {
	client *redis.Client
	script *redis.Script
}

func NewFixedWindowLimiter(client *redis.Client) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		client: client,
		script: redis.NewScript(fixedWindowScript),
	}
}

func (l *FixedWindowLimiter) Allow(ctx context.Context, req CheckRequest) (*Result, error) {
	key := fmt.Sprintf("ratelimit:fixed_window:%s:%s", req.RouteID, req.ClientID)
	
	res, err := l.script.Run(ctx, l.client, []string{key}, req.Limit, req.WindowSeconds).Result()
	if err != nil {
		return nil, err
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) != 2 {
		return nil, fmt.Errorf("unexpected script result format")
	}

	allowedInt, _ := resSlice[0].(int64)
	currentInt, _ := resSlice[1].(int64)

	allowed := allowedInt == 1
	remaining := req.Limit - int(currentInt)
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:   allowed,
		Remaining: remaining,
	}, nil
}
