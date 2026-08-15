-- KEYS[1] = key, ARGV[1] = limit, ARGV[2] = window_seconds
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[2])
end
if current > tonumber(ARGV[1]) then
    return {0, current}
else
    return {1, current}
end
