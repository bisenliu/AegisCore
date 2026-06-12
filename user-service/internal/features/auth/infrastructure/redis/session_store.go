package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/workerpool"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
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
	// deleteAllUserSessionsPurgeWorkers 限制退出全部设备后台清理并发。
	deleteAllUserSessionsPurgeWorkers = 4
	// deleteAllUserSessionsPurgeStopTimeout 限制服务关闭时等待后台清理的时间。
	deleteAllUserSessionsPurgeStopTimeout = 30 * time.Second

	// expiredSessionMinScore 让 ZRemRangeByScore 清理所有 score 小于等于当前时间的会话。
	expiredSessionMinScore = "-inf"
)

const (
	createSessionResultOK            int64 = 1
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

var createSessionScript = rediscache.NewScript(`
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

	Redis     *rediscache.Client `name:"cache_redis"`
	Cfg       *config.Config
	PurgePool PurgeTaskPool `name:"auth_session_purge_pool"`
}

type SessionStore struct {
	redis                *rediscache.Client
	keys                 KeyCatalog
	tokenVersionCacheTTL time.Duration
	purgePool            PurgeTaskPool
}

// PurgeTaskPool 是认证 Redis 适配器消费的后台清理任务池窄接口。
type PurgeTaskPool interface {
	Submit(ctx context.Context, task workerpool.Task) error
	Stats() workerpool.Stats
}

// NewSessionStore 构造认证会话持久化的 Redis 实现。
func NewSessionStore(params SessionStoreParams) (*SessionStore, error) {
	keys, err := NewKeyCatalog(params.Cfg.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new auth redis keys: %w", err)
	}
	return &SessionStore{
		redis:                params.Redis,
		keys:                 keys,
		tokenVersionCacheTTL: params.Cfg.Auth.TokenVersionCacheTTL,
		purgePool:            params.PurgePool,
	}, nil
}

// GetCachedTokenVersion 返回缓存的 token version，未命中时返回 ErrTokenVersionCacheMiss。
func (r *SessionStore) GetCachedTokenVersion(ctx context.Context, userID string) (int64, error) {
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
func (r *SessionStore) CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
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
func (r *SessionStore) DeleteCachedTokenVersion(ctx context.Context, userID string) error {
	if err := r.redis.Del(ctx, r.tokenVersionKey(userID)).Err(); err != nil {
		return fmt.Errorf("delete token version cache: %w", err)
	}
	return nil
}

// CreateSession 存储 refresh token 会话，并按用户建立索引用于批量撤销。
func (r *SessionStore) CreateSession(ctx context.Context, session authdomain.AuthSession, ttl time.Duration, maxActiveSessionsPerUser int) error {
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

	result, err := createSessionScript.Run(ctx, r.redis, []string{r.sessionKey(session.UserID, session.SessionID), userSessions},
		data,
		milliseconds(ttl),
		session.SessionID,
		redisScore(expiresAt),
		redisScore(now),
		milliseconds(indexTTL),
		r.keys.AuthSessionPrefix(session.UserID),
		strconv.Itoa(maxActiveSessionsPerUser),
	).Int64()
	if err != nil {
		return fmt.Errorf("create auth session: %w", err)
	}
	if result != createSessionResultOK {
		return fmt.Errorf("create auth session: unexpected script result %d", result)
	}
	return nil
}

// RotateSession 原子消费旧 refresh 会话，并创建新 refresh 会话。
func (r *SessionStore) RotateSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, ttl time.Duration, maxActiveSessionsPerUser int) error {
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
		redisScore(expiresAt),
		redisScore(now),
		milliseconds(indexTTL),
		r.keys.AuthSessionPrefix(newSession.UserID),
		strconv.Itoa(maxActiveSessionsPerUser),
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
func (r *SessionStore) GetSession(ctx context.Context, userID string, sessionID string) (authdomain.AuthSession, error) {
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
func (r *SessionStore) DeleteSession(ctx context.Context, userID string, sessionID string) error {
	userSessions := r.userSessionsKey(userID)
	pipe := r.redis.TxPipeline()
	pipe.Del(ctx, r.sessionKey(userID, sessionID))
	pipe.ZRemRangeByScore(ctx, userSessions, expiredSessionMinScore, redisScore(time.Now()))
	pipe.ZRem(ctx, userSessions, sessionID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}
	return nil
}

// DeleteAllUserSessions 删除用户所有存活 refresh token 会话，并删除用户索引。
func (r *SessionStore) DeleteAllUserSessions(ctx context.Context, userID string) error {
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
		sessionPrefix := r.keys.AuthSessionPrefix(userID)
		task := workerpool.Task{
			Name: "auth.redis.purge_detached_user_sessions",
			Fields: []zap.Field{
				zap.String("user_id", userID),
				zap.String("purge_key", purgeKey),
				zap.String("session_prefix", sessionPrefix),
				zap.Time("cut_time", cutTime),
				zap.Int64("batch_size", deleteAllUserSessionsBatchSize),
			},
			Run: func(taskCtx context.Context) error {
				purgeCtx, cancel := context.WithTimeout(taskCtx, deleteAllUserSessionsPurgeTTL)
				defer cancel()
				return r.purgeDetachedUserSessions(purgeCtx, purgeKey, sessionPrefix, cutTime)
			},
		}
		if err := r.purgePool.Submit(context.WithoutCancel(ctx), task); err != nil {
			return fmt.Errorf("submit delete user auth sessions purge: %w", err)
		}
		return nil
	case detachUserSessionsResultConflict:
		return fmt.Errorf("delete user auth sessions: purge key conflict")
	default:
		return fmt.Errorf("delete user auth sessions: unexpected script result %d", result)
	}
}

func (r *SessionStore) purgeDetachedUserSessions(ctx context.Context, purgeKey string, sessionPrefix string, cutTime time.Time) error {
	cutScore := redisScoreFloat(cutTime)
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

func (r *SessionStore) tokenVersionKey(userID string) string {
	return r.keys.AuthUserTokenVersion(userID)
}

func (r *SessionStore) sessionKey(userID string, sessionID string) string {
	return r.keys.AuthSession(userID, sessionID)
}

func (r *SessionStore) userSessionsKey(userID string) string {
	return r.keys.AuthUserSessions(userID)
}

func (r *SessionStore) purgeUserSessionsKey(userID string) string {
	return r.keys.AuthUserSessionsPurge(userID, uuid.NewString())
}

func redisScore(t time.Time) string {
	return strconv.FormatFloat(redisScoreFloat(t), 'f', 9, 64)
}

func redisScoreFloat(t time.Time) float64 {
	return float64(t.UnixNano()) / float64(time.Second)
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
