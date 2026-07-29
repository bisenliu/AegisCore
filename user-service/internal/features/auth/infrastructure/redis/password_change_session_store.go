package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

const (
	consumePasswordChangeSessionResultOK       int64 = 1
	consumePasswordChangeSessionResultNotFound int64 = 2
	consumePasswordChangeSessionResultMismatch int64 = 3
)

// CreatePasswordChangeSession 存储强制改密一次性会话。
func (r *SessionStore) CreatePasswordChangeSession(ctx context.Context, session authdomain.PasswordChangeSession, ttl time.Duration) error {
	if ttl <= 0 {
		// 非正数 TTL 回退到短期默认值，避免创建永久一次性凭据。
		ttl = defaultPasswordChangeSessionTTL
	}
	session.ExpiresAt = time.Now().Add(ttl)
	data, err := json.Marshal(newPasswordChangeSessionPayload(session))
	if err != nil {
		return fmt.Errorf("marshal password change session: %w", err)
	}
	created, err := r.redis.SetNX(ctx, r.passwordChangeSessionKey(session.UserID, session.SessionID), data, ttl).Result()
	if err != nil {
		return fmt.Errorf("create password change session: %w", err)
	}
	if !created {
		return fmt.Errorf("create password change session: key exists")
	}
	return nil
}

// ConsumePasswordChangeSession 原子消费强制改密一次性会话。
func (r *SessionStore) ConsumePasswordChangeSession(ctx context.Context, expected authdomain.PasswordChangeSession) error {
	result, err := consumePasswordChangeSessionScript.Run(ctx, r.redis, []string{r.passwordChangeSessionKey(expected.UserID, expected.SessionID)},
		expected.UserID.String(),
		expected.SessionID,
		expected.TokenID,
		formatTokenVersion(expected.TokenVersion),
	).Int64()
	if err != nil {
		return fmt.Errorf("consume password change session: %w", err)
	}
	switch result {
	case consumePasswordChangeSessionResultOK:
		return nil
	case consumePasswordChangeSessionResultNotFound:
		r.metricsRecorder().PasswordChangeSessionConsumeFailed(ctx, authapplication.MetricsPasswordChangeReasonNotFound)
		r.metricsRecorder().PasswordChangeSessionReuseRejected(ctx)
		return authdomain.ErrPasswordChangeSessionNotFound
	case consumePasswordChangeSessionResultMismatch:
		r.metricsRecorder().PasswordChangeSessionConsumeFailed(ctx, authapplication.MetricsPasswordChangeReasonMismatch)
		return authdomain.ErrPasswordChangeSessionMismatch
	default:
		r.metricsRecorder().PasswordChangeSessionConsumeFailed(ctx, authapplication.MetricsPasswordChangeReasonSystemError)
		return fmt.Errorf("consume password change session: unexpected script result %d", result)
	}
}

// RevokePasswordChangeSession 删除未消费的强制改密一次性会话。
func (r *SessionStore) RevokePasswordChangeSession(ctx context.Context, userID uuid.UUID, sessionID string) error {
	if err := r.redis.Del(ctx, r.passwordChangeSessionKey(userID, sessionID)).Err(); err != nil {
		return fmt.Errorf("revoke password change session: %w", err)
	}
	return nil
}
