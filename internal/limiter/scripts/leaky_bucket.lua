-- KEYS[1] = key, ARGV[1] = now_ms, ARGV[2] = capacity, ARGV[3] = leak_rate_per_sec
local bucket = redis.call("HMGET", KEYS[1], "queue_level", "last_leak")
local queue_level = tonumber(bucket[1])
local last_leak = tonumber(bucket[2])
if queue_level == nil then
    queue_level = 0
    last_leak = tonumber(ARGV[1])
end
local delta_ms = math.max(0, tonumber(ARGV[1]) - last_leak)
local leaked = (delta_ms / 1000) * tonumber(ARGV[3])
queue_level = math.max(0, queue_level - leaked)

local allowed = 0
if queue_level + 1 <= tonumber(ARGV[2]) then
    queue_level = queue_level + 1
    allowed = 1
end

redis.call("HMSET", KEYS[1], "queue_level", queue_level, "last_leak", ARGV[1])
redis.call("EXPIRE", KEYS[1], 3600)

local remaining = math.floor(tonumber(ARGV[2]) - queue_level)
if remaining < 0 then
    remaining = 0
end
return {allowed, remaining}
