-- KEYS[1] = key, ARGV[1] = now_ms, ARGV[2] = window_ms, ARGV[3] = limit, ARGV[4] = member_suffix
local clear_before = tonumber(ARGV[1]) - tonumber(ARGV[2])
redis.call("ZREMRANGEBYSCORE", KEYS[1], 0, clear_before)
local count = redis.call("ZCARD", KEYS[1])
if count < tonumber(ARGV[3]) then
    redis.call("ZADD", KEYS[1], ARGV[1], ARGV[1] .. "-" .. ARGV[4])
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
    return {1, count + 1}
else
    return {0, count}
end
