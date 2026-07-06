-- name: auth.consume_password_change_session
-- contract: KEYS[1]=password_change_session_key, ARGV[1]=user_id, ARGV[2]=session_id, ARGV[3]=token_id, ARGV[4]=token_version
-- version: 1
-- returns: 1=consumed, 2=not_found, 3=mismatch

local data = redis.call("GET", KEYS[1])
if not data then
  return 2
end

local ok, session = pcall(cjson.decode, data)
if not ok then
  return 3
end

if session["user_id"] ~= ARGV[1] then
  return 3
end
if session["session_id"] ~= ARGV[2] then
  return 3
end
if session["token_id"] ~= ARGV[3] then
  return 3
end
if tostring(session["token_version"]) ~= ARGV[4] then
  return 3
end

redis.call("DEL", KEYS[1])
return 1
