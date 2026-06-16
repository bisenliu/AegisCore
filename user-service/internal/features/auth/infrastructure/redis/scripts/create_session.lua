-- name: auth.create_session
-- contract: KEYS[1]=session_key, KEYS[2]=user_sessions_zset
-- version: 1
-- returns:
--   1 = ok

redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
redis.call("ZREMRANGEBYSCORE", KEYS[2], "-inf", ARGV[5])
redis.call("ZADD", KEYS[2], ARGV[4], ARGV[3])

local max_sessions = tonumber(ARGV[8])
if max_sessions and max_sessions > 0 then
	local overflow = redis.call("ZCARD", KEYS[2]) - max_sessions
	if overflow > 0 then
		local stale_sessions = redis.call("ZRANGE", KEYS[2], 0, overflow - 1)
		for _, session_id in ipairs(stale_sessions) do
			redis.call("DEL", ARGV[7] .. session_id)
			redis.call("ZREM", KEYS[2], session_id)
		end
	end
end

local index_ttl = redis.call("PTTL", KEYS[2])
local target_ttl = tonumber(ARGV[6])
if index_ttl < target_ttl then
	redis.call("PEXPIRE", KEYS[2], target_ttl)
end

return 1
