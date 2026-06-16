-- name: auth.detach_user_sessions
-- contract: KEYS[1]=user_sessions_zset, KEYS[2]=purge_user_sessions_zset, ARGV[1]=purge_ttl_seconds
-- version: 1
-- returns:
--   0 = empty
--   1 = detached
--   2 = conflict

if redis.call("EXISTS", KEYS[1]) == 0 then
	return 0
end
if redis.call("EXISTS", KEYS[2]) == 1 then
	return 2
end
redis.call("RENAME", KEYS[1], KEYS[2])
redis.call("EXPIRE", KEYS[2], ARGV[1])
return 1
