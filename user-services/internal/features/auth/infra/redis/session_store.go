package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aegiscore/common/runtime/config"
	authdomain "github.com/aegiscore/user-services/internal/features/auth/domain"
	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

const (
	// defaultTokenVersionCacheTTL 限制 token_version 校验依赖 Redis 缓存的最长时间。
	defaultTokenVersionCacheTTL = 5 * time.Minute
	// defaultAuthSessionTTL 是调用方未提供有效时长时的会话兜底过期时间。
	defaultAuthSessionTTL = time.Hour
	// authSessionIndexTTLBuffer 让用户会话索引在最后一个会话过期后仍保留短暂窗口用于懒清理。
	authSessionIndexTTLBuffer = 5 * time.Minute
	// deleteAllUserSessionsPurgeTTL 限制临时清理索引在后台清理失败时的最长保留时间。
	deleteAllUserSessionsPurgeTTL = time.Hour
	// deleteAllUserSessionsBatchSize 限制退出全部设备后台清理每批 Redis key 数量。
	deleteAllUserSessionsBatchSize int64 = 500

	// expiredSessionMinScore 让 ZRemRangeByScore 清理所有 score 小于等于当前时间的会话。
	expiredSessionMinScore = "-inf"
)

const (
	rotateSessionResultOK            int64 = 1
	rotateSessionResultNotFound      int64 = 2
	rotateSessionResultMismatch      int64 = 3
	cacheTokenVersionResultStored          = 1
	cacheTokenVersionResultSkipped         = 2
	detachUserSessionsResultEmpty          = 0
	detachUserSessionsResultDetached       = 1
	detachUserSessionsResultConflict       = 2
)

var cacheTokenVersionScript = rediscache.NewScript(`
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
`)

var rotateSessionScript = rediscache.NewScript(`
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

local index_ttl = redis.call("PTTL", KEYS[3])
local target_ttl = tonumber(ARGV[10])
if index_ttl < target_ttl then
	redis.call("PEXPIRE", KEYS[3], target_ttl)
end

return 1
`)

var detachUserSessionsScript = rediscache.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 0 then
	return 0
end
if redis.call("EXISTS", KEYS[2]) == 1 then
	return 2
end
redis.call("RENAME", KEYS[1], KEYS[2])
redis.call("EXPIRE", KEYS[2], ARGV[1])
return 1
`)

// SessionStoreParams 包含 Redis 认证会话 store 所需的 Fx 输入。
type SessionStoreParams struct {
	fx.In

	Redis *rediscache.Client `name:"cache_redis"`
	Cfg   *config.Config
	Keys  authdomain.RedisKeyBuilder
}

type sessionStore struct {
	redis                *rediscache.Client
	keys                 authdomain.RedisKeyBuilder
	tokenVersionCacheTTL time.Duration
}

// NewSessionStore 构造认证会话持久化的 Redis 实现。
func NewSessionStore(params SessionStoreParams) *sessionStore {
	return &sessionStore{redis: params.Redis, keys: params.Keys, tokenVersionCacheTTL: params.Cfg.Auth.TokenVersionCacheTTL}
}

// GetCachedTokenVersion 返回缓存的 token version，未命中时返回 ErrTokenVersionCacheMiss。
func (r *sessionStore) GetCachedTokenVersion(ctx context.Context, userID string) (int64, error) {
	key := r.tokenVersionKey(userID)
	value, err := r.redis.Get(ctx, key).Result()
	if errors.Is(err, rediscache.Nil) {
		return 0, authdomain.ErrTokenVersionCacheMiss
	}
	if err != nil {
		return 0, fmt.Errorf("get token version cache: %w", err)
	}
	version, err := parseTokenVersion(value)
	if err != nil || version <= 0 {
		return 0, authdomain.ErrTokenVersionCacheMiss
	}
	return version, nil
}

// CacheTokenVersion 存储用户 token version，供中间件执行撤销校验。
func (r *sessionStore) CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	ttl := r.tokenVersionCacheTTL
	if ttl <= 0 {
		// 非正数配置表示使用有界默认过期窗口，而不是创建永久缓存项。
		ttl = defaultTokenVersionCacheTTL
	}
	result, err := cacheTokenVersionScript.Run(ctx, r.redis, []string{r.tokenVersionKey(userID)}, formatTokenVersion(tokenVersion), milliseconds(ttl)).Int64()
	if err != nil {
		return fmt.Errorf("set token version cache: %w", err)
	}
	if result != cacheTokenVersionResultStored && result != cacheTokenVersionResultSkipped {
		return fmt.Errorf("set token version cache: unexpected script result %d", result)
	}
	return nil
}

// DeleteCachedTokenVersion 删除用户 token version 缓存，使后续校验回源 PostgreSQL。
func (r *sessionStore) DeleteCachedTokenVersion(ctx context.Context, userID string) error {
	if err := r.redis.Del(ctx, r.tokenVersionKey(userID)).Err(); err != nil {
		return fmt.Errorf("delete token version cache: %w", err)
	}
	return nil
}

// CreateSession 存储 refresh token 会话，并按用户建立索引用于批量撤销。
func (r *sessionStore) CreateSession(ctx context.Context, session authdomain.AuthSession, ttl time.Duration) error {
	if ttl <= 0 {
		// 非正数 TTL 回退到短期会话，避免创建永久 Redis key。
		ttl = defaultAuthSessionTTL
	}
	now := time.Now()
	expiresAt := now.Add(ttl)
	session.ExpiresAt = expiresAt
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal auth session: %w", err)
	}
	userSessions := r.userSessionsKey(session.UserID)
	indexTTL := ttl + authSessionIndexTTLBuffer
	indexCurrentTTL, err := r.redis.TTL(ctx, userSessions).Result()
	if err != nil {
		return fmt.Errorf("get user auth sessions ttl: %w", err)
	}
	pipe := r.redis.TxPipeline()
	pipe.Set(ctx, r.sessionKey(session.UserID, session.SessionID), data, ttl)
	pipe.ZRemRangeByScore(ctx, userSessions, expiredSessionMinScore, unixScore(now))
	pipe.ZAdd(ctx, userSessions, rediscache.Z{Score: float64(expiresAt.Unix()), Member: session.SessionID})
	if indexCurrentTTL < indexTTL {
		// 用户会话索引需要长于最长会话，但不应被新会话缩短有效期。
		pipe.Expire(ctx, userSessions, indexTTL)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create auth session: %w", err)
	}
	return nil
}

// RotateSession 原子消费旧 refresh 会话，并创建新 refresh 会话。
func (r *sessionStore) RotateSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, ttl time.Duration) error {
	if newSession.UserID != oldSession.UserID || newSession.TokenVersion != oldSession.TokenVersion {
		return authdomain.ErrAuthSessionMismatch
	}
	if ttl <= 0 {
		// 非正数 TTL 回退到短期会话，避免创建永久 Redis key。
		ttl = defaultAuthSessionTTL
	}
	now := time.Now()
	expiresAt := now.Add(ttl)
	newSession.ExpiresAt = expiresAt
	data, err := json.Marshal(newSession)
	if err != nil {
		return fmt.Errorf("marshal rotated auth session: %w", err)
	}
	oldKey := r.sessionKey(oldSession.UserID, oldSession.SessionID)
	newKey := r.sessionKey(newSession.UserID, newSession.SessionID)
	userSessions := r.userSessionsKey(newSession.UserID)
	indexTTL := ttl + authSessionIndexTTLBuffer

	result, err := rotateSessionScript.Run(ctx, r.redis, []string{oldKey, newKey, userSessions},
		oldSession.UserID,
		oldSession.SessionID,
		formatTokenVersion(oldSession.TokenVersion),
		newSession.SessionID,
		formatTokenVersion(newSession.TokenVersion),
		data,
		milliseconds(ttl),
		unixScore(expiresAt),
		unixScore(now),
		milliseconds(indexTTL),
	).Int64()
	if err != nil {
		return fmt.Errorf("rotate auth session: %w", err)
	}
	switch result {
	case rotateSessionResultOK:
		return nil
	case rotateSessionResultNotFound:
		return authdomain.ErrAuthSessionNotFound
	case rotateSessionResultMismatch:
		return authdomain.ErrAuthSessionMismatch
	default:
		return fmt.Errorf("rotate auth session: unexpected script result %d", result)
	}
}

// GetSession 按 session ID 返回 refresh token 会话。
func (r *sessionStore) GetSession(ctx context.Context, userID string, sessionID string) (authdomain.AuthSession, error) {
	data, err := r.redis.Get(ctx, r.sessionKey(userID, sessionID)).Bytes()
	if errors.Is(err, rediscache.Nil) {
		return authdomain.AuthSession{}, authdomain.ErrAuthSessionNotFound
	}
	if err != nil {
		return authdomain.AuthSession{}, fmt.Errorf("get auth session: %w", err)
	}
	var session authdomain.AuthSession
	if err := json.Unmarshal(data, &session); err != nil {
		return authdomain.AuthSession{}, fmt.Errorf("unmarshal auth session: %w", err)
	}
	return session, nil
}

// DeleteSession 删除一个 refresh token 会话，并从用户索引中清理过期项。
func (r *sessionStore) DeleteSession(ctx context.Context, userID string, sessionID string) error {
	userSessions := r.userSessionsKey(userID)
	pipe := r.redis.TxPipeline()
	pipe.Del(ctx, r.sessionKey(userID, sessionID))
	pipe.ZRemRangeByScore(ctx, userSessions, expiredSessionMinScore, unixScore(time.Now()))
	pipe.ZRem(ctx, userSessions, sessionID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}
	return nil
}

// DeleteAllUserSessions 删除用户所有存活 refresh token 会话，并删除用户索引。
func (r *sessionStore) DeleteAllUserSessions(ctx context.Context, userID string) error {
	userSessions := r.userSessionsKey(userID)
	purgeKey := r.purgeUserSessionsKey(userID)
	cutTime := time.Now()
	result, err := detachUserSessionsScript.Run(ctx, r.redis, []string{userSessions, purgeKey}, seconds(deleteAllUserSessionsPurgeTTL)).Int64()
	if err != nil {
		return fmt.Errorf("delete user auth sessions: %w", err)
	}
	switch result {
	case detachUserSessionsResultEmpty:
		return nil
	case detachUserSessionsResultDetached:
		go func() {
			purgeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deleteAllUserSessionsPurgeTTL)
			defer cancel()
			_ = r.purgeDetachedUserSessions(purgeCtx, purgeKey, r.keys.AuthSessionPrefix(userID), cutTime)
		}()
		return nil
	case detachUserSessionsResultConflict:
		return fmt.Errorf("delete user auth sessions: purge key conflict")
	default:
		return fmt.Errorf("delete user auth sessions: unexpected script result %d", result)
	}
}

func (r *sessionStore) purgeDetachedUserSessions(ctx context.Context, purgeKey string, sessionPrefix string, cutTime time.Time) error {
	cutScore := float64(cutTime.Unix())
	for {
		sessions, err := r.redis.ZRangeWithScores(ctx, purgeKey, 0, deleteAllUserSessionsBatchSize-1).Result()
		if err != nil {
			return fmt.Errorf("read detached user sessions: %w", err)
		}
		if len(sessions) == 0 {
			break
		}

		sessionKeys := make([]string, 0, len(sessions))
		sessionIDs := make([]interface{}, 0, len(sessions))
		for _, session := range sessions {
			sessionID := redisMemberString(session.Member)
			sessionIDs = append(sessionIDs, sessionID)
			if session.Score > cutScore {
				sessionKeys = append(sessionKeys, sessionPrefix+sessionID)
			}
		}
		if len(sessionKeys) > 0 {
			if err := r.redis.Unlink(ctx, sessionKeys...).Err(); err != nil {
				return fmt.Errorf("unlink detached auth sessions: %w", err)
			}
		}
		if err := r.redis.ZRem(ctx, purgeKey, sessionIDs...).Err(); err != nil {
			return fmt.Errorf("remove detached user sessions: %w", err)
		}
	}
	if err := r.redis.Unlink(ctx, purgeKey).Err(); err != nil {
		return fmt.Errorf("unlink detached user sessions index: %w", err)
	}
	return nil
}

func (r *sessionStore) tokenVersionKey(userID string) string {
	return r.keys.AuthUserTokenVersion(userID)
}

func (r *sessionStore) sessionKey(userID string, sessionID string) string {
	return r.keys.AuthSession(userID, sessionID)
}

func (r *sessionStore) userSessionsKey(userID string) string {
	return r.keys.AuthUserSessions(userID)
}

func (r *sessionStore) purgeUserSessionsKey(userID string) string {
	return r.keys.AuthUserSessionsPurge(userID, uuid.NewString())
}

func unixScore(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

func seconds(ttl time.Duration) string {
	return strconv.FormatInt(int64(ttl/time.Second), 10)
}

func milliseconds(ttl time.Duration) string {
	return strconv.FormatInt(ttl.Milliseconds(), 10)
}

func parseTokenVersion(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func formatTokenVersion(version int64) string {
	return strconv.FormatInt(version, 10)
}

func redisMemberString(member interface{}) string {
	switch value := member.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return fmt.Sprint(value)
	}
}
