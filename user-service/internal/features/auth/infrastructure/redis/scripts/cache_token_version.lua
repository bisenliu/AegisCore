-- name: auth.cache_token_version
-- contract: KEYS[1]=token_version_key, ARGV[1]=next_token_version, ARGV[2]=ttl_milliseconds
-- version: 1
-- returns:
--   1 = stored
--   2 = skipped_newer_value

local current = redis.call("GET", KEYS[1])
local next_version = tonumber(ARGV[1])
if current then
	local current_version = tonumber(current)
	if current_version and current_version > next_version then
		return 2
	end
end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
return 1
