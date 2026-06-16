-- name: auth.rotate_session
-- contract: KEYS[1]=old_session_key, KEYS[2]=new_session_key, KEYS[3]=user_sessions_zset
-- version: 1
-- returns:
--   1 = ok
--   2 = old_session_not_found
--   3 = old_session_mismatch

local old_payload = redis.call("GET", KEYS[1])
if not old_payload then
	return 2
end

local ok, old_session = pcall(cjson.decode, old_payload)
if not ok then
	return 3
end
if old_session["user_id"] ~= ARGV[1] or old_session["session_id"] ~= ARGV[2] or tostring(old_session["token_version"]) ~= ARGV[3] then
	return 3
end

redis.call("SET", KEYS[2], ARGV[6], "PX", ARGV[7])
redis.call("ZREMRANGEBYSCORE", KEYS[3], "-inf", ARGV[9])
redis.call("ZADD", KEYS[3], ARGV[8], ARGV[4])
redis.call("ZREM", KEYS[3], ARGV[2])
redis.call("DEL", KEYS[1])

local max_sessions = tonumber(ARGV[12])
if max_sessions and max_sessions > 0 then
	local overflow = redis.call("ZCARD", KEYS[3]) - max_sessions
	if overflow > 0 then
		local stale_sessions = redis.call("ZRANGE", KEYS[3], 0, overflow - 1)
		for _, session_id in ipairs(stale_sessions) do
			redis.call("DEL", ARGV[11] .. session_id)
			redis.call("ZREM", KEYS[3], session_id)
		end
	end
end

local index_ttl = redis.call("PTTL", KEYS[3])
local target_ttl = tonumber(ARGV[10])
if index_ttl < target_ttl then
	redis.call("PEXPIRE", KEYS[3], target_ttl)
end

return 1
