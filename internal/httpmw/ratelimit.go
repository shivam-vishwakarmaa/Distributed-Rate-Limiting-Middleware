package httpmw

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"rate-limiter/internal/breaker"
	"rate-limiter/internal/config"
	"rate-limiter/internal/fallback"
	"rate-limiter/internal/limiter"
	"rate-limiter/internal/metrics"

	"github.com/redis/go-redis/v9"
)

type Middleware struct {
	config       *config.Config
	limiters     map[string]*breaker.CircuitBreaker
	trustedCIDRs []*net.IPNet
}

func NewMiddleware(cfg *config.Config, rdb *redis.Client) *Middleware {
	localLimiter := fallback.NewLocalLimiter()

	limiters := make(map[string]*breaker.CircuitBreaker)

	algos := map[string]limiter.Limiter{
		"fixed_window":       limiter.NewFixedWindowLimiter(rdb),
		"sliding_window_log": limiter.NewSlidingWindowLogLimiter(rdb),
		"token_bucket":       limiter.NewTokenBucketLimiter(rdb),
		"leaky_bucket":       limiter.NewLeakyBucketLimiter(rdb),
	}

	for algoName, primary := range algos {
		cb := breaker.NewCircuitBreaker(
			algoName,
			primary,
			localLimiter,
			cfg.CircuitBreaker.FailureThreshold,
			cfg.CircuitBreaker.OpenDurationSeconds,
		)
		limiters[algoName] = cb
	}

	trustedCIDRs := parseTrustedCIDRs(os.Getenv("TRUSTED_PROXY_CIDRS"))

	return &Middleware{
		config:       cfg,
		limiters:     limiters,
		trustedCIDRs: trustedCIDRs,
	}
}

func (m *Middleware) GetCircuitBreakerState() string {
	for _, cb := range m.limiters {
		state := cb.State()
		if state == breaker.StateOpen || state == breaker.StateHalfOpen {
			return string(state)
		}
	}
	return string(breaker.StateClosed)
}

func (m *Middleware) GetCircuitBreakers() map[string]*breaker.CircuitBreaker {
	return m.limiters
}

func parseTrustedCIDRs(envStr string) []*net.IPNet {
	if envStr == "" {
		return nil
	}
	parts := strings.Split(envStr, ",")
	var cidrs []*net.IPNet
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		_, network, err := net.ParseCIDR(p)
		if err == nil {
			cidrs = append(cidrs, network)
		}
	}
	return cidrs
}

func (m *Middleware) resolveClientID(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	clientIP := net.ParseIP(host)

	isTrusted := false
	for _, cidr := range m.trustedCIDRs {
		if cidr.Contains(clientIP) {
			isTrusted = true
			break
		}
	}

	if isTrusted {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0]) // use the original client
		}
	}

	return host
}

func (m *Middleware) matchRoute(path string) *config.RouteConfig {
	var bestMatch *config.RouteConfig
	longestPrefix := -1

	for i := range m.config.Routes {
		r := &m.config.Routes[i]
		if strings.HasPrefix(path, r.PathPrefix) {
			if len(r.PathPrefix) > longestPrefix {
				longestPrefix = len(r.PathPrefix)
				bestMatch = r
			}
		}
	}
	
	if bestMatch == nil && len(m.config.Routes) > 0 {
		bestMatch = &m.config.Routes[len(m.config.Routes)-1]
	}

	return bestMatch
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeCfg := m.matchRoute(r.URL.Path)
		if routeCfg == nil {
			next.ServeHTTP(w, r)
			return
		}

		cb, ok := m.limiters[routeCfg.Algorithm]
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		clientID := m.resolveClientID(r)
		req := limiter.CheckRequest{
			RouteID:         routeCfg.PathPrefix,
			ClientID:        clientID,
			Limit:           routeCfg.Limit,
			WindowSeconds:   routeCfg.WindowSeconds,
			Capacity:        routeCfg.Capacity,
			RefillPerSecond: routeCfg.RefillPerSecond,
		}

		start := time.Now()
		res, err := cb.Allow(r.Context(), req)
		duration := time.Since(start).Seconds()
		metrics.CheckDuration.Observe(duration)

		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		limit := routeCfg.Limit
		if routeCfg.Algorithm == "token_bucket" || routeCfg.Algorithm == "leaky_bucket" {
			limit = routeCfg.Capacity
		}

		if !res.Allowed {
			metrics.RequestsDenied.WithLabelValues(routeCfg.PathPrefix, routeCfg.Algorithm).Inc()
			w.Header().Set("Retry-After", strconv.Itoa(routeCfg.WindowSeconds))
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.Itoa(routeCfg.WindowSeconds))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)

			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":         "rate_limit_exceeded",
				"limit":         limit,
				"remaining":     0,
				"reset_seconds": routeCfg.WindowSeconds,
			})
			return
		}

		metrics.RequestsAllowed.WithLabelValues(routeCfg.PathPrefix, routeCfg.Algorithm).Inc()
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.Itoa(routeCfg.WindowSeconds))

		next.ServeHTTP(w, r)
	})
}
