package redis

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

type authSessionPayload struct {
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	TokenVersion int64     `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type passwordChangeSessionPayload struct {
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	TokenID      string    `json:"token_id"`
	TokenVersion int64     `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func newAuthSessionPayload(session authdomain.AuthSession) authSessionPayload {
	return authSessionPayload{
		UserID:       session.UserID.String(),
		SessionID:    session.SessionID,
		TokenVersion: session.TokenVersion,
		ExpiresAt:    session.ExpiresAt,
	}
}

func (p authSessionPayload) domainSession() (authdomain.AuthSession, error) {
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return authdomain.AuthSession{}, fmt.Errorf("parse auth session user id: %w", err)
	}
	return authdomain.AuthSession{
		UserID:       userID,
		SessionID:    p.SessionID,
		TokenVersion: p.TokenVersion,
		ExpiresAt:    p.ExpiresAt,
	}, nil
}

func newPasswordChangeSessionPayload(session authdomain.PasswordChangeSession) passwordChangeSessionPayload {
	return passwordChangeSessionPayload{
		UserID:       session.UserID.String(),
		SessionID:    session.SessionID,
		TokenID:      session.TokenID,
		TokenVersion: session.TokenVersion,
		ExpiresAt:    session.ExpiresAt,
	}
}
