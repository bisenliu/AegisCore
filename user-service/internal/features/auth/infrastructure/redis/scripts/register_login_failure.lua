-- name: auth.register_login_failure
-- contract: KEYS[1]=login_failure_key, ARGV[1]=max_failures, ARGV[2]=failure_window_milliseconds, ARGV[3]=lockout_ttl_milliseconds
-- version: 1
-- returns: failures, locked, ttl_milliseconds
-- locked: 0 = counting, 1 = locked

local max_failures = tonumber(ARGV[1])
local failure_window = tonumber(ARGV[2])
local lockout_ttl = tonumber(ARGV[3])
local current = redis.call("GET", KEYS[1])
local failures = 0

-- locked 是独立标记。只递增数字会让后续读取分不清“还在计数”和“已经锁定”。
if current == "locked" then
	local ttl = redis.call("PTTL", KEYS[1])
	if ttl < 0 then
		ttl = 0
	end
	return {max_failures, 1, ttl}
end

local current_ttl = redis.call("PTTL", KEYS[1])

if current then
	failures = tonumber(current) or 0
end
failures = failures + 1

local locked = 0
if failures >= max_failures then
	-- 达到阈值后把同一 key 改成锁定标记，并把剩余时间改为锁定时长。锁定到期即视为计数失效。
	redis.call("SET", KEYS[1], "locked", "PX", lockout_ttl)
	locked = 1
else
	-- 未达到锁定阈值：首次失败开启观察窗口；后续失败沿用原有观察窗口的剩余时间，避免因 SET 抹除 TTL 或延长未锁定计数窗口。
	if current_ttl > 0 then
		redis.call("SET", KEYS[1], tostring(failures), "PX", current_ttl)
	else
		redis.call("SET", KEYS[1], tostring(failures), "PX", failure_window)
	end
end

local ttl = redis.call("PTTL", KEYS[1])
if ttl < 0 then
	ttl = 0
end
return {failures, locked, ttl}
