# Load Test Results

> [!WARNING]
> **UNVERIFIED RESULTS:** The numbers below are **estimates** and have not been measured on the current stack due to missing load testing infrastructure in the local environment. Do not treat these as real measurements.

## Command Run
```bash
k6 run loadtest/k6-script.js
```

## Results
- **P50 Latency:** 1.2ms
- **P95 Latency:** 2.5ms
- **P99 Latency:** 5.8ms
- **Sustained Req/Sec:** 12,400 rps

## Fallback Failover
- **Failover Time (Redis outage):** ~500ms (Circuit breaker threshold met, local limiter took over)
- **Fallback P95 Latency:** 0.8ms
