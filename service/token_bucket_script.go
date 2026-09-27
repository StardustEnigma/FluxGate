package service

const tokenBucketScript=`
local key=KEYS[1]

local capacity=tonumber(ARGV[1])
local refillRate=tonumber(ARGV[2])
local now=tonumber(ARGV[3])

local data =redis.call("HGETALL",key)

local currentTokens
local lastRefill
if #data == 0 then
	currentTokens=capacity
	lastRefill=now
else 
	currentTokens = tonumber(redis.call("HGET",key,"currentTokens"))
	lastRefill = tonumber(redis.call("HGET",key,"lastRefill"))
end

local elapsed= (now-lastRefill)/1000000000

local newTokens=elapsed * refillRate

if newTokens > 0 then 
	currentTokens = math.min(
	currentTokens + newTokens ,
	capacity 
	)
	lastRefill= now
end

local allowed=0
local retryAfter=0

if currentTokens >= 1 then 
	currentTokens=currentTokens-1
	allowed=1
else 
	retryAfter=((1-currentTokens)/refillRate)*1000
end

redis.call(
	"HSET",
	key,
	"capacity",capacity,
	"refillRate",refillRate,
	"currentTokens",currentTokens,
	"lastRefill",lastRefill
)

return {
	allowed,
	currentTokens,
	retryAfter
	}
`