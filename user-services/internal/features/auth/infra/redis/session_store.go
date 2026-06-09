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

	// expiredSessionMinScore 让 ZRemRangeByScore 清理所有 score 小于等于当前时间的会话。
	expiredSessionMinScore = "-inf"
)

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
	if err := r.redis.Set(ctx, r.tokenVersionKey(userID), formatTokenVersion(tokenVersion), ttl).Err(); err != nil {
		return fmt.Errorf("set token version cache: %w", err)
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
	pipe.Set(ctx, r.sessionKey(session.SessionID), data, ttl)
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
	oldKey := r.sessionKey(oldSession.SessionID)
	newKey := r.sessionKey(newSession.SessionID)
	userSessions := r.userSessionsKey(newSession.UserID)
	indexTTL := ttl + authSessionIndexTTLBuffer
	err = r.redis.Watch(ctx, func(tx *rediscache.Tx) error {
		stored, err := getWatchedSession(ctx, tx, oldKey)
		if err != nil {
			return err
		}
		if stored.UserID != oldSession.UserID || stored.SessionID != oldSession.SessionID || stored.TokenVersion != oldSession.TokenVersion {
			return authdomain.ErrAuthSessionMismatch
		}
		indexCurrentTTL, err := tx.TTL(ctx, userSessions).Result()
		if err != nil {
			return fmt.Errorf("get user auth sessions ttl: %w", err)
		}
		_, err = tx.TxPipelined(ctx, func(pipe rediscache.Pipeliner) error {
			pipe.Set(ctx, newKey, data, ttl)
			pipe.ZRemRangeByScore(ctx, userSessions, expiredSessionMinScore, unixScore(now))
			pipe.ZAdd(ctx, userSessions, rediscache.Z{Score: float64(expiresAt.Unix()), Member: newSession.SessionID})
			pipe.ZRem(ctx, userSessions, oldSession.SessionID)
			pipe.Del(ctx, oldKey)
			if indexCurrentTTL < indexTTL {
				pipe.Expire(ctx, userSessions, indexTTL)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("rotate auth session: %w", err)
		}
		return nil
	}, oldKey)
	if errors.Is(err, rediscache.TxFailedErr) {
		return authdomain.ErrAuthSessionNotFound
	}
	if err != nil {
		return err
	}
	return nil
}

// GetSession 按 session ID 返回 refresh token 会话。
func (r *sessionStore) GetSession(ctx context.Context, sessionID string) (authdomain.AuthSession, error) {
	data, err := r.redis.Get(ctx, r.sessionKey(sessionID)).Bytes()
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
	pipe.Del(ctx, r.sessionKey(sessionID))
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
	if err := r.redis.ZRemRangeByScore(ctx, userSessions, expiredSessionMinScore, unixScore(time.Now())).Err(); err != nil {
		return fmt.Errorf("clean expired user auth sessions: %w", err)
	}
	sessions, err := r.redis.ZRange(ctx, userSessions, 0, -1).Result()
	if err != nil && !errors.Is(err, rediscache.Nil) {
		return fmt.Errorf("list user auth sessions: %w", err)
	}
	pipe := r.redis.TxPipeline()
	// 已先清理过期成员，因此只需显式删除存活会话的载荷 key。
	for _, sessionID := range sessions {
		pipe.Del(ctx, r.sessionKey(sessionID))
	}
	pipe.Del(ctx, userSessions)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete user auth sessions: %w", err)
	}
	return nil
}

func (r *sessionStore) tokenVersionKey(userID string) string {
	return r.keys.AuthUserTokenVersion(userID)
}

func (r *sessionStore) sessionKey(sessionID string) string {
	return r.keys.AuthSession(sessionID)
}

func (r *sessionStore) userSessionsKey(userID string) string {
	return r.keys.AuthUserSessions(userID)
}

func getWatchedSession(ctx context.Context, tx *rediscache.Tx, key string) (authdomain.AuthSession, error) {
	data, err := tx.Get(ctx, key).Bytes()
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

func unixScore(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

func parseTokenVersion(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func formatTokenVersion(version int64) string {
	return strconv.FormatInt(version, 10)
}
