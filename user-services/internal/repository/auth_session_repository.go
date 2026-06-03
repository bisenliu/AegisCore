package repository

import (
	"context"
	"errors"
	"time"
)

var ErrAuthSessionNotFound = errors.New("auth session not found")
var ErrTokenVersionMismatch = errors.New("token version mismatch")

type AuthSession struct {
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	TokenVersion int64     `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type AuthSessionRepository interface {
	GetCurrentTokenVersion(ctx context.Context, userID string) (int64, error)
	ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
	CreateSession(ctx context.Context, session AuthSession, ttl time.Duration) error
	GetSession(ctx context.Context, sessionID string) (AuthSession, error)
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	DeleteAllUserSessions(ctx context.Context, userID string) error
	InvalidateUserTokenVersion(ctx context.Context, userID string) error
}
