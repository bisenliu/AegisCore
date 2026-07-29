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
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/workerpool"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

const (
	createSessionResultOK       int64 = 1
	rotateSessionResultOK       int64 = 1
	rotateSessionResultNotFound       = 2
	rotateSessionResultMismatch       = 3

	detachUserSessionsResultEmpty    = 0
	detachUserSessionsResultDetached = 1
	detachUserSessionsResultConflict = 2
)

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
		r.keys.AuthSessionPrefix(session.UserID.String()),
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
		oldSession.UserID.String(),
		oldSession.SessionID,
		formatTokenVersion(oldSession.TokenVersion),
		newSession.SessionID,
		formatTokenVersion(newSession.TokenVersion),
		data,
		milliseconds(ttl),
		redisScore(expiresAt),
		redisScore(now),
		milliseconds(indexTTL),
		r.keys.AuthSessionPrefix(newSession.UserID.String()),
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
func (r *SessionStore) GetSession(ctx context.Context, userID uuid.UUID, sessionID string) (authdomain.AuthSession, error) {
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
func (r *SessionStore) DeleteSession(ctx context.Context, userID uuid.UUID, sessionID string) error {
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

// DeleteAllUserSessions 从在线路径摘除用户 session 索引，并后台清理已摘除 session key。
// 两阶段删除避免一次性阻塞请求线程；purge 只处理 cutTime 前索引里的会话，避免误删撤销开始后新建的并发会话。
func (r *SessionStore) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error {
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
		sessionPrefix := r.keys.AuthSessionPrefix(userID.String())
		task := workerpool.Task{
			Name: "auth.redis.purge_detached_user_sessions",
			Fields: []zap.Field{
				zap.String("user_id", userID.String()),
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

		// score 早于 cutTime 的条目可能是旧索引中的过期残留；只删除 cutTime 之后仍有效的 session key，并始终移除 purge 索引项。
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
