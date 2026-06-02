package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

var ErrSessionNotFound = errors.New("auth session not found")
var ErrTokenVersionMismatch = errors.New("token version mismatch")

type Session struct {
	UserID       int64     `json:"user_id"`
	SessionID    string    `json:"session_id"`
	TokenVersion int64     `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type SessionStore interface {
	GetCurrentTokenVersion(ctx context.Context, userID int64) (int64, error)
	ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
	CreateSession(ctx context.Context, session Session, ttl time.Duration) error
	GetSession(ctx context.Context, sessionID string) (Session, error)
	DeleteSession(ctx context.Context, userID int64, sessionID string) error
	DeleteAllUserSessions(ctx context.Context, userID int64) error
	InvalidateUserTokenVersion(ctx context.Context, userID int64) error
}

type SessionStoreParams struct {
	fx.In

	Redis *redis.Client `name:"cache_redis"`
	Repo  repository.UserRepository
	Cfg   *config.Config
}

type redisSessionStore struct {
	redis                *redis.Client
	repo                 repository.UserRepository
	tokenVersionCacheTTL time.Duration
}

func NewSessionStore(params SessionStoreParams) SessionStore {
	return &redisSessionStore{redis: params.Redis, repo: params.Repo, tokenVersionCacheTTL: params.Cfg.Auth.TokenVersionCacheTTL}
}

func (s *redisSessionStore) GetCurrentTokenVersion(ctx context.Context, userID int64) (int64, error) {
	key := tokenVersionKey(userID)
	value, err := s.redis.Get(ctx, key).Result()
	if err == nil {
		version, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr == nil && version > 0 {
			return version, nil
		}
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("get token version cache: %w", err)
	}

	version, err := s.repo.GetTokenVersion(ctx, userID)
	if err != nil {
		return 0, err
	}
	ttl := s.tokenVersionCacheTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if err := s.redis.Set(ctx, key, strconv.FormatInt(version, 10), ttl).Err(); err != nil {
		return 0, fmt.Errorf("set token version cache: %w", err)
	}
	return version, nil
}

func (s *redisSessionStore) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}
	current, err := s.GetCurrentTokenVersion(ctx, id)
	if err != nil {
		return err
	}
	if current != tokenVersion {
		return ErrTokenVersionMismatch
	}
	return nil
}

func (s *redisSessionStore) CreateSession(ctx context.Context, session Session, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = time.Hour
	}
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal auth session: %w", err)
	}
	pipe := s.redis.TxPipeline()
	pipe.Set(ctx, sessionKey(session.SessionID), data, ttl)
	pipe.SAdd(ctx, userSessionsKey(session.UserID), session.SessionID)
	pipe.Expire(ctx, userSessionsKey(session.UserID), ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create auth session: %w", err)
	}
	return nil
}

func (s *redisSessionStore) GetSession(ctx context.Context, sessionID string) (Session, error) {
	data, err := s.redis.Get(ctx, sessionKey(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get auth session: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("unmarshal auth session: %w", err)
	}
	return session, nil
}

func (s *redisSessionStore) DeleteSession(ctx context.Context, userID int64, sessionID string) error {
	pipe := s.redis.TxPipeline()
	pipe.Del(ctx, sessionKey(sessionID))
	pipe.SRem(ctx, userSessionsKey(userID), sessionID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}
	return nil
}

func (s *redisSessionStore) DeleteAllUserSessions(ctx context.Context, userID int64) error {
	sessions, err := s.redis.SMembers(ctx, userSessionsKey(userID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("list user auth sessions: %w", err)
	}
	pipe := s.redis.TxPipeline()
	for _, sessionID := range sessions {
		pipe.Del(ctx, sessionKey(sessionID))
	}
	pipe.Del(ctx, userSessionsKey(userID))
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete user auth sessions: %w", err)
	}
	return nil
}

func (s *redisSessionStore) InvalidateUserTokenVersion(ctx context.Context, userID int64) error {
	if err := s.redis.Del(ctx, tokenVersionKey(userID)).Err(); err != nil {
		return fmt.Errorf("delete token version cache: %w", err)
	}
	return nil
}

func tokenVersionKey(userID int64) string {
	return fmt.Sprintf("auth:user:%d:token_version", userID)
}

func sessionKey(sessionID string) string {
	return fmt.Sprintf("auth:session:%s", sessionID)
}

func userSessionsKey(userID int64) string {
	return fmt.Sprintf("auth:user:%d:sessions", userID)
}
