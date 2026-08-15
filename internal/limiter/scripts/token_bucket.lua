-- KEYS[1] = key, ARGV[1] = now_ms, ARGV[2] = capacity, ARGV[3] = refill_per_sec
local bucket = redis.call("HMGET", KEYS[1], "tokens", "last_refill")
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])
if tokens == nil then
    tokens = tonumber(ARGV[2])
    last_refill = tonumber(ARGV[1])
end
local delta_ms = math.max(0, tonumber(ARGV[1]) - last_refill)
tokens = math.min(tonumber(ARGV[2]), tokens + (delta_ms / 1000) * tonumber(ARGV[3]))
local allowed = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
end
redis.call("HMSET", KEYS[1], "tokens", tokens, "last_refill", ARGV[1])
redis.call("EXPIRE", KEYS[1], 3600)
return {allowed, math.floor(tokens)}
