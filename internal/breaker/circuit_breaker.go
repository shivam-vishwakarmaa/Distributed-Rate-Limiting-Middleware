package breaker

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"rate-limiter/internal/limiter"
)

type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

type CircuitBreaker struct {
	mu                  sync.RWMutex
	state               State
	failures            int
	failureThreshold    int
	openDuration        time.Duration
	lastFailureTime     time.Time
	primaryLimiter      limiter.Limiter
	fallbackLimiter     limiter.Limiter
	name                string
}

func NewCircuitBreaker(name string, primary, fallback limiter.Limiter, failureThreshold, openDurationSec int) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		state:            StateClosed,
		failureThreshold: failureThreshold,
		openDuration:     time.Duration(openDurationSec) * time.Second,
		primaryLimiter:   primary,
		fallbackLimiter:  fallback,
	}
}

func (cb *CircuitBreaker) logTransition(from, to State) {
	msg := map[string]string{
		"event": "circuit_breaker_transition",
		"name":  cb.name,
		"from":  string(from),
		"to":    string(to),
	}
	b, _ := json.Marshal(msg)
	log.Println(string(b))
}

func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) > cb.openDuration {
			return StateHalfOpen
		}
	}
	return cb.state
}

func (cb *CircuitBreaker) Allow(ctx context.Context, req limiter.CheckRequest) (*limiter.Result, error) {
	state := cb.State()

	if state == StateOpen {
		return cb.fallbackLimiter.Allow(ctx, req)
	}

	if state == StateHalfOpen {
		cb.mu.Lock()
		if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.openDuration {
			cb.state = StateHalfOpen
		}
		
		if cb.state != StateHalfOpen {
			cb.mu.Unlock()
			return cb.Allow(ctx, req)
		}
		// In half-open, let one request through to Redis
		cb.mu.Unlock()
	}

	res, err := cb.primaryLimiter.Allow(ctx, req)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailureTime = time.Now()
		
		prevState := cb.state
		if cb.state == StateClosed && cb.failures >= cb.failureThreshold {
			cb.state = StateOpen
		} else if cb.state == StateHalfOpen {
			cb.state = StateOpen
		}
		
		if prevState != cb.state {
			cb.logTransition(prevState, cb.state)
		}
		
		return cb.fallbackLimiter.Allow(ctx, req)
	}

	if cb.state == StateHalfOpen {
		cb.logTransition(StateHalfOpen, StateClosed)
		cb.state = StateClosed
		cb.failures = 0
	} else if cb.state == StateClosed {
		cb.failures = 0
	}

	return res, nil
}
