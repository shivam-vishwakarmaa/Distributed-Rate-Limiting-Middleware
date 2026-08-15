package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"rate-limiter/internal/breaker"
	"rate-limiter/internal/config"
	"rate-limiter/internal/httpmw"
	"rate-limiter/internal/metrics"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		DialTimeout:  time.Duration(cfg.Redis.DialTimeoutMs) * time.Millisecond,
		ReadTimeout:  time.Duration(cfg.Redis.CommandTimeoutMs) * time.Millisecond,
		WriteTimeout: time.Duration(cfg.Redis.CommandTimeoutMs) * time.Millisecond,
	})

	mw := httpmw.NewMiddleware(cfg, rdb)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			for algo, cb := range mw.GetCircuitBreakers() {
				state := cb.State()
				val := 0.0
				if state == breaker.StateHalfOpen {
					val = 1.0
				} else if state == breaker.StateOpen {
					val = 2.0
				}
				metrics.CircuitState.WithLabelValues(algo).Set(val)
			}
		}
	}()

	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 1*time.Second)
		defer cancel()

		redisStatus := "ok"
		if err := rdb.Ping(ctx).Err(); err != nil {
			redisStatus = "down"
		}

		cbState := mw.GetCircuitBreakerState()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"redis_circuit": cbState,
			"redis_ping":    redisStatus,
		})
	})

	r.Handle("/metrics", metrics.Handler())

	backendURLStr := os.Getenv("BACKEND_URL")
	if backendURLStr == "" {
		backendURLStr = "http://localhost:8081"
	}
	backendURL, err := url.Parse(backendURLStr)
	if err != nil {
		log.Fatalf("Invalid backend URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	r.Route("/api", func(r chi.Router) {
		r.Use(mw.Handler)
		r.Handle("/*", proxy)
	})
	
	r.With(mw.Handler).Handle("/*", proxy)

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
