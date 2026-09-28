package service

const slidingWindowScript = `
local key =KEYS[1]
local now =tonumber(ARGV[1])
local window =tonumber(ARGV[2])
local limit =tonumber(ARGV[3])
local request_id=ARGV[4]

local window_start=now - window

redis.call(
	"ZREMRANGEBYSCORE",
	key,
	"-inf",
	window_start
)
local count =redis.call(
	"ZCARD",
	key
)

if count >=limit then
	local oldest = redis.call(
		"ZRANGE",
		key,
		0,
		0,
		"WITHSCORES"
	)
	local oldest_time = tonumber(oldest[2])
	local retry_after=(oldest_time + window ) - now
	return {0,retry_after}
end 

redis.call(
	"ZADD",
	key,
	now,
	request_id
)
return {1,0}

`