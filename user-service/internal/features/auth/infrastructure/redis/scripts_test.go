package redis

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedisLuaScriptsDeclareContracts(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		metadata []string
	}{
		{
			name:   "cache token version",
			source: cacheTokenVersionLua,
			metadata: []string{
				"-- name: auth.cache_token_version",
				"-- contract: KEYS[1]=token_version_key, ARGV[1]=next_token_version, ARGV[2]=ttl_milliseconds",
				"-- version: 1",
			},
		},
		{
			name:   "create session",
			source: createSessionLua,
			metadata: []string{
				"-- name: auth.create_session",
				"-- contract: KEYS[1]=session_key, KEYS[2]=user_sessions_zset",
				"-- version: 1",
			},
		},
		{
			name:   "rotate session",
			source: rotateSessionLua,
			metadata: []string{
				"-- name: auth.rotate_session",
				"-- contract: KEYS[1]=old_session_key, KEYS[2]=new_session_key, KEYS[3]=user_sessions_zset",
				"-- version: 1",
			},
		},
		{
			name:   "detach user sessions",
			source: detachUserSessionsLua,
			metadata: []string{
				"-- name: auth.detach_user_sessions",
				"-- contract: KEYS[1]=user_sessions_zset, KEYS[2]=purge_user_sessions_zset, ARGV[1]=purge_ttl_seconds",
				"-- version: 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, metadata := range tt.metadata {
				require.True(t, strings.Contains(tt.source, metadata),
					"script metadata %q not found", metadata)

			}
		})
	}
}
