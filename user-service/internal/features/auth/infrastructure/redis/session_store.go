package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	rediscache "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/workerpool"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
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

// SessionStoreParams 包含 Redis 认证会话 store 所需的 Fx 输入。
type SessionStoreParams struct {
	fx.In

	Redis     *rediscache.Client `name:"cache_redis"`
	Cfg       *config.Config
	PurgePool PurgeTaskPool           `name:"auth_session_purge_pool"`
	Metrics   authapplication.Metrics `optional:"true"`
}

type SessionStore struct {
	redis                *rediscache.Client
	keys                 KeyCatalog
	tokenVersionCacheTTL time.Duration
	purgePool            PurgeTaskPool
	metrics              authapplication.Metrics
}

var (
	_ authapplication.TokenVersionCache   = (*SessionStore)(nil)
	_ authapplication.RefreshSessionStore = (*SessionStore)(nil)
)

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
	metrics := params.Metrics
	if metrics == nil {
		metrics = authapplication.NopMetrics()
	}
	return &SessionStore{
		redis:                params.Redis,
		keys:                 keys,
		tokenVersionCacheTTL: params.Cfg.Auth.TokenVersionCacheTTL,
		purgePool:            params.PurgePool,
		metrics:              metrics,
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
	purgeKey, err := r.purgeUserSessionsKey(userID)
	if err != nil {
		return err
	}
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
			r.metricsRecorder().SessionPurgeSubmitFailed(ctx)
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

func (r *SessionStore) metricsRecorder() authapplication.Metrics {
	if r == nil || r.metrics == nil {
		return authapplication.NopMetrics()
	}
	return r.metrics
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

func (r *SessionStore) purgeUserSessionsKey(userID string) (string, error) {
	purgeID, err := runtimeid.NewUUIDString()
	if err != nil {
		return "", fmt.Errorf("generate auth sessions purge id: %w", err)
	}
	return r.keys.AuthUserSessionsPurge(userID, purgeID), nil
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
