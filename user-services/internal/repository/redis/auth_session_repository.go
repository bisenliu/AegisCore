package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/aegiscore/user-services/internal/service"
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

type AuthSessionRepositoryParams struct {
	fx.In

	Redis *rediscache.Client `name:"cache_redis"`
	Cfg   *config.Config
	Keys  service.RedisKeyBuilder
}

type authSessionRepository struct {
	redis                *rediscache.Client
	keys                 service.RedisKeyBuilder
	tokenVersionCacheTTL time.Duration
}

func NewAuthSessionRepository(params AuthSessionRepositoryParams) repository.AuthSessionRepository {
	return &authSessionRepository{redis: params.Redis, keys: params.Keys, tokenVersionCacheTTL: params.Cfg.Auth.TokenVersionCacheTTL}
}

func (r *authSessionRepository) GetCachedTokenVersion(ctx context.Context, userID string) (int64, error) {
	key := r.tokenVersionKey(userID)
	value, err := r.redis.Get(ctx, key).Result()
	if errors.Is(err, rediscache.Nil) {
		return 0, repository.ErrTokenVersionCacheMiss
	}
	if err != nil {
		return 0, fmt.Errorf("get token version cache: %w", err)
	}
	version, err := parseTokenVersion(value)
	if err != nil || version <= 0 {
		return 0, repository.ErrTokenVersionCacheMiss
	}
	return version, nil
}

func (r *authSessionRepository) CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	ttl := r.tokenVersionCacheTTL
	if ttl <= 0 {
		ttl = defaultTokenVersionCacheTTL
	}
	if err := r.redis.Set(ctx, r.tokenVersionKey(userID), formatTokenVersion(tokenVersion), ttl).Err(); err != nil {
		return fmt.Errorf("set token version cache: %w", err)
	}
	return nil
}

func (r *authSessionRepository) CreateSession(ctx context.Context, session repository.AuthSession, ttl time.Duration) error {
	if ttl <= 0 {
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
		pipe.Expire(ctx, userSessions, indexTTL)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create auth session: %w", err)
	}
	return nil
}

func (r *authSessionRepository) GetSession(ctx context.Context, sessionID string) (repository.AuthSession, error) {
	data, err := r.redis.Get(ctx, r.sessionKey(sessionID)).Bytes()
	if errors.Is(err, rediscache.Nil) {
		return repository.AuthSession{}, repository.ErrAuthSessionNotFound
	}
	if err != nil {
		return repository.AuthSession{}, fmt.Errorf("get auth session: %w", err)
	}
	var session repository.AuthSession
	if err := json.Unmarshal(data, &session); err != nil {
		return repository.AuthSession{}, fmt.Errorf("unmarshal auth session: %w", err)
	}
	return session, nil
}

func (r *authSessionRepository) DeleteSession(ctx context.Context, userID string, sessionID string) error {
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

func (r *authSessionRepository) DeleteAllUserSessions(ctx context.Context, userID string) error {
	userSessions := r.userSessionsKey(userID)
	if err := r.redis.ZRemRangeByScore(ctx, userSessions, expiredSessionMinScore, unixScore(time.Now())).Err(); err != nil {
		return fmt.Errorf("clean expired user auth sessions: %w", err)
	}
	sessions, err := r.redis.ZRange(ctx, userSessions, 0, -1).Result()
	if err != nil && !errors.Is(err, rediscache.Nil) {
		return fmt.Errorf("list user auth sessions: %w", err)
	}
	pipe := r.redis.TxPipeline()
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

func (r *authSessionRepository) tokenVersionKey(userID string) string {
	return r.keys.AuthUserTokenVersion(userID)
}

func (r *authSessionRepository) sessionKey(sessionID string) string {
	return r.keys.AuthSession(sessionID)
}

func (r *authSessionRepository) userSessionsKey(userID string) string {
	return r.keys.AuthUserSessions(userID)
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
