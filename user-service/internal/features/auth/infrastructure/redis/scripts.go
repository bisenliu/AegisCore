package redis

import (
	_ "embed"

	rediscache "github.com/redis/go-redis/v9"
)

//go:embed scripts/cache_token_version.lua
var cacheTokenVersionLua string

//go:embed scripts/create_session.lua
var createSessionLua string

//go:embed scripts/rotate_session.lua
var rotateSessionLua string

//go:embed scripts/detach_user_sessions.lua
var detachUserSessionsLua string

//go:embed scripts/consume_password_change_session.lua
var consumePasswordChangeSessionLua string

var (
	cacheTokenVersionScript            = rediscache.NewScript(cacheTokenVersionLua)
	createSessionScript                = rediscache.NewScript(createSessionLua)
	rotateSessionScript                = rediscache.NewScript(rotateSessionLua)
	detachUserSessionsScript           = rediscache.NewScript(detachUserSessionsLua)
	consumePasswordChangeSessionScript = rediscache.NewScript(consumePasswordChangeSessionLua)
)
