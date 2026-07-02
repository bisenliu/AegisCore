package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var authTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestAuthUseCaseLogin(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	credential := normalCredential()
	issued := issuedTokenPair("access", "refresh", 900, time.Hour)

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(credential, nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ int64, sessionID string) (*authtokens.IssuedTokenPair, error) {
			if sessionID == "" {
				t.Fatal("sessionID is empty")
			}
			return issued, nil
		})
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), time.Hour).
		DoAndReturn(func(_ context.Context, _ string, sessionID string, _ int64, _ time.Duration) error {
			if sessionID == "" {
				t.Fatal("sessionID is empty")
			}
			return nil
		})

	tokens, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken != "access" || tokens.RefreshToken != "refresh" || tokens.TokenType != commonauth.TokenTypeBearer || tokens.ExpiresIn != 900 {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAuthUseCaseLoginRecordsMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", 900, time.Hour), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), time.Hour).Return(nil)
	metrics.EXPECT().LoginSucceeded(gomock.Any())

	_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
}

func TestAuthUseCaseLoginRecordsFailureReasons(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		metrics := NewMockMetrics(ctrl)
		fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
		metrics.EXPECT().LoginFailed(gomock.Any(), authapplication.MetricsReasonValidationFailed)

		_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: " "})
		if !errors.Is(err, authdomain.ErrInvalidCredentials) {
			t.Fatalf("err = %v, want invalid credentials", err)
		}
	})

	t.Run("status rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		metrics := NewMockMetrics(ctrl)
		fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
		fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(nil, errors.Join(authdomain.ErrInvalidCredentials, authdomain.ErrUserStatusRejected))
		metrics.EXPECT().LoginFailed(gomock.Any(), authapplication.MetricsReasonUserStatusRejected)

		_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
		if !errors.Is(err, authdomain.ErrInvalidCredentials) {
			t.Fatalf("err = %v, want invalid credentials", err)
		}
	})

	t.Run("kdf busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		metrics := NewMockMetrics(ctrl)
		fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
		fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(nil, password.ErrPasswordKDFBusy)
		metrics.EXPECT().LoginFailed(gomock.Any(), authapplication.MetricsReasonPasswordKDFBusy)

		tokens, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
		if !errors.Is(err, password.ErrPasswordKDFBusy) {
			t.Fatalf("err = %v, want ErrPasswordKDFBusy", err)
		}
		if tokens != nil {
			t.Fatalf("tokens = %#v, want nil", tokens)
		}
	})
}

func TestAuthUseCaseLoginRejectsBlankTrimmedCredentials(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)

	_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: " "})

	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want authdomain.ErrInvalidCredentials", err)
	}
}

func TestAuthUseCaseLoginUsesDefaultTTLs(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	defaultAccessTokenTTL := 15 * time.Minute
	defaultRefreshTokenTTL := 7 * 24 * time.Hour

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", int64(defaultAccessTokenTTL.Seconds()), defaultRefreshTokenTTL), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), defaultRefreshTokenTTL).Return(nil)

	tokens, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.ExpiresIn != int64(defaultAccessTokenTTL.Seconds()) {
		t.Fatalf("ExpiresIn = %d, want %d", tokens.ExpiresIn, int64(defaultAccessTokenTTL.Seconds()))
	}
}

func TestAuthUseCaseLoginUsesExplicitTTLs(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	accessTTL := time.Minute
	refreshTTL := 2 * time.Hour

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", int64(accessTTL.Seconds()), refreshTTL), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), refreshTTL).Return(nil)

	tokens, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.ExpiresIn != int64(accessTTL.Seconds()) {
		t.Fatalf("ExpiresIn = %d, want %d", tokens.ExpiresIn, int64(accessTTL.Seconds()))
	}
}

func TestAuthUseCaseLoginPassesMaxActiveSessionsPerUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	credentials := NewMockVerifier(ctrl)
	tokens := NewMockIssuer(ctrl)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenVersions := NewMockTokenVersionCache(ctrl)
	sessions := NewMockRefreshSessionStore(ctrl)
	cfg := config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: true, TokenVersionCacheTTL: time.Minute, MaxActiveSessionsPerUser: 3}
	svc := NewLoginUseCase(LoginDeps{
		Credentials: credentials,
		Tokens:      tokens,
		Sessions:    authsessions.NewLifecycle(users, tokenVersions, sessions, cfg.MaxActiveSessionsPerUser),
	})

	credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", 900, time.Hour), nil)
	sessions.EXPECT().CreateSession(gomock.Any(), gomock.Any(), time.Hour, 3).DoAndReturn(func(_ context.Context, session authdomain.AuthSession, _ time.Duration, _ int) error {
		if session.UserID != authTestUserID.String() || session.SessionID == "" || session.TokenVersion != 2 {
			t.Fatalf("session = %#v", session)
		}
		return nil
	})

	_, err := svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
}

func TestAuthUseCaseLoginDoesNotReturnTokenWhenSessionCreateFails(t *testing.T) {
	createErr := errors.New("create session failed")
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", 900, time.Hour), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), time.Hour).Return(createErr)

	tokens, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

	if !errors.Is(err, createErr) {
		t.Fatalf("Login err = %v, want create session error", err)
	}
	if tokens != nil {
		t.Fatalf("tokens = %#v, want nil", tokens)
	}
}

func TestAuthUseCaseLoginRejectsInvalidCredentials(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "wrong").Return(nil, authdomain.ErrInvalidCredentials)

	_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "wrong"})

	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want authdomain.ErrInvalidCredentials", err)
	}
}

func TestAuthUseCaseLoginRejectsInactiveStatuses(t *testing.T) {
	for _, status := range []identity.UserStatus{identity.UserStatusDisabled} {
		fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
		fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(nil, errors.Join(authdomain.ErrInvalidCredentials, authdomain.ErrUserStatusRejected))

		_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

		if !errors.Is(err, authdomain.ErrInvalidCredentials) {
			t.Fatalf("status %d err = %v, want authdomain.ErrInvalidCredentials", status, err)
		}
	}
}

func TestAuthUseCaseLoginIssuesPasswordChangeToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	credential := &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", Status: identity.UserStatusMustChangePassword, TokenVersion: 2}
	passwordChange := &authtokens.TokenResult{AccessToken: "password-change", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900, PasswordChangeRequired: true}

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(credential, nil)
	fixture.tokens.EXPECT().IssuePasswordChangeToken(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(passwordChange, nil)

	tokens, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken != "password-change" || tokens.RefreshToken != "" || !tokens.PasswordChangeRequired {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAuthUseCaseChangePassword(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	gomock.InOrder(
		fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil),
		fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(nil),
		fixture.credentials.EXPECT().ChangePassword(gomock.Any(), authTestUserID, "new-secret").Return(&authdomain.CredentialUpdateResult{UserID: authTestUserID, TokenVersion: 3}, nil),
		fixture.sessions.EXPECT().RevokeUserSessionsAtVersion(gomock.Any(), authTestUserID, int64(3)).Return(nil),
	)

	result, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if result == nil || !result.Changed {
		t.Fatalf("result=%#v", result)
	}
}

func TestAuthUseCaseChangePasswordIncrementsTokenVersionOnce(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(nil)
	fixture.credentials.EXPECT().ChangePassword(gomock.Any(), authTestUserID, "new-secret").Return(&authdomain.CredentialUpdateResult{UserID: authTestUserID, TokenVersion: 3}, nil).Times(1)
	fixture.sessions.EXPECT().RevokeUserSessionsAtVersion(gomock.Any(), authTestUserID, int64(3)).Return(nil).Times(1)

	_, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
}

func TestAuthUseCaseChangePasswordSucceedsWhenRevocationProjectionFails(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(nil)
	fixture.credentials.EXPECT().ChangePassword(gomock.Any(), authTestUserID, "new-secret").Return(&authdomain.CredentialUpdateResult{UserID: authTestUserID, TokenVersion: 3}, nil)
	fixture.sessions.EXPECT().RevokeUserSessionsAtVersion(gomock.Any(), authTestUserID, int64(3)).Return(errors.New("projection failed"))

	result, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if result == nil || !result.Changed {
		t.Fatalf("result = %#v", result)
	}
}

func TestAuthUseCaseChangePasswordMapsCredentialUpdateNotFound(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(nil)
	fixture.credentials.EXPECT().ChangePassword(gomock.Any(), authTestUserID, "new-secret").Return(nil, identity.ErrUserNotFound)

	_, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})

	if !errors.Is(err, identity.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestAuthUseCaseChangePasswordMapsTokenVersionUserNotFound(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(identity.ErrUserNotFound)

	_, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})

	if !errors.Is(err, identity.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestAuthUseCaseChangePasswordRejectsAccessToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "access").Return(nil, uuid.Nil, authdomain.ErrTokenInvalid)

	_, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "access", NewPassword: "new-secret"})

	if !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
	}
}

func TestAuthUseCaseRefreshRotatesSession(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}

	gomock.InOrder(
		fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "Bearer refresh").Return(claims, nil),
		fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil),
		fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil),
		fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).DoAndReturn(func(_ context.Context, _ authdomain.AuthSession, newSession authdomain.AuthSession, _ time.Duration) error {
			if newSession.UserID != authTestUserID.String() || newSession.SessionID == "" || newSession.SessionID == "s-old" || newSession.TokenVersion != 2 {
				t.Fatalf("new session = %#v", newSession)
			}
			return nil
		}),
	)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: commonauth.TokenPrefix + "refresh"})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken != "access" || tokens.RefreshToken != "refresh-new" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAuthUseCaseRefreshRotationPassesMaxActiveSessionsPerUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokens := NewMockIssuer(ctrl)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenVersions := NewMockTokenVersionCache(ctrl)
	sessions := NewMockRefreshSessionStore(ctrl)
	cfg := config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, TokenVersionCacheTTL: time.Minute, RefreshTokenRotation: true, MaxActiveSessionsPerUser: 4}
	svc := NewRefreshTokenUseCase(RefreshTokenDeps{
		Tokens:   tokens,
		Sessions: authsessions.NewLifecycle(users, tokenVersions, sessions, cfg.MaxActiveSessionsPerUser),
		Config:   &config.Config{Auth: cfg},
	})
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}

	tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	sessions.EXPECT().GetSession(gomock.Any(), authTestUserID.String(), "s-old").Return(oldSession, nil)
	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(2), nil)
	tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	sessions.EXPECT().RotateSession(gomock.Any(), oldSession, gomock.Any(), time.Hour, 4).Return(nil)

	_, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
}

func TestAuthUseCaseRefreshRejectsInvalidNormalizedToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(false), nil)

	for _, token := range []string{"", " ", commonauth.TokenTypeBearer, commonauth.TokenPrefix} {
		_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: token})

		if !errors.Is(err, authdomain.ErrTokenInvalid) {
			t.Fatalf("token %q err = %v, want authdomain.ErrTokenInvalid", token, err)
		}
	}
}

func TestAuthUseCaseRefreshRotationKeepsOldSessionWhenTokenSigningFails(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
	signErr := errors.New("sign failed")

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(nil, signErr)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if !errors.Is(err, signErr) {
		t.Fatalf("Refresh err = %v, want sign error", err)
	}
}

func TestAuthUseCaseRefreshRotationKeepsOldSessionWhenNewSessionCreateFails(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
	createErr := errors.New("create failed")

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(createErr)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if !errors.Is(err, createErr) {
		t.Fatalf("Refresh err = %v, want create error", err)
	}
	if tokens != nil {
		t.Fatalf("tokens = %#v, want nil", tokens)
	}
}

func TestAuthUseCaseRefreshRotationFailureDoesNotReturnToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
	rotateErr := errors.New("rotate failed")

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(rotateErr)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if !errors.Is(err, rotateErr) {
		t.Fatalf("Refresh err = %v, want rotate error", err)
	}
	if tokens != nil {
		t.Fatalf("tokens = %#v, want nil", tokens)
	}
}

func TestAuthUseCaseRefreshRotationReturnsTokenAfterNewSessionAndOldRevocation(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(nil)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens == nil || tokens.AccessToken != "access" || tokens.RefreshToken != "refresh-new" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAuthUseCaseRefreshUsesNormalizedToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(false), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), "s-old").Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), "s-old", int64(2), time.Hour).Return(nil)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken != "access" || tokens.RefreshToken != "refresh-new" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAuthUseCaseRefreshRejectsAccessTokenSubject(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(false), nil)
	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "access").Return(nil, authdomain.ErrTokenInvalid)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "access"})

	if !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
	}
}

func TestAuthUseCaseRefreshRejectsVersionChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	claims := refreshClaims("s-old", 2)

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(authdomain.AuthSession{}, int64(0), errors.Join(authdomain.ErrTokenInvalid, commonauth.ErrTokenVersionMismatch))
	metrics.EXPECT().RefreshFailed(gomock.Any(), authapplication.MetricsReasonTokenVersionMismatch)
	metrics.EXPECT().TokenVersionMismatch(gomock.Any(), authapplication.MetricsSourceRefreshToken)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
	}
}

func TestAuthUseCaseRefreshMapsTokenVersionUserNotFound(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(authdomain.AuthSession{}, int64(0), identity.ErrUserNotFound)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if !errors.Is(err, identity.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestAuthUseCaseLogoutCurrentDeletesSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID.String(), "s-123").Return(nil)
	metrics.EXPECT().LogoutSucceeded(gomock.Any(), authapplication.MetricsOperationLogoutCurrent)

	result, err := fixture.LogoutCurrentSession(ctx)

	if err != nil {
		t.Fatalf("LogoutCurrentSession: %v", err)
	}
	if result == nil || !result.LoggedOut {
		t.Fatalf("result = %#v", result)
	}
}

func TestAuthUseCaseLogoutCurrentRecordsDeleteFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")
	deleteErr := errors.New("delete failed")

	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID.String(), "s-123").Return(deleteErr)
	metrics.EXPECT().LogoutFailed(gomock.Any(), authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonSessionDeleteFailed)

	_, err := fixture.LogoutCurrentSession(ctx)

	if !errors.Is(err, deleteErr) {
		t.Fatalf("err = %v, want delete error", err)
	}
}

func TestAuthUseCaseLogoutAllIncrementsVersionAndDeletesSessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")
	revocation := &authdomain.SessionRevocationResult{UserID: authTestUserID, TokenVersion: 3}

	fixture.sessions.EXPECT().RevokeAllUserSessions(gomock.Any(), authTestUserID).Return(revocation, nil)
	metrics.EXPECT().LogoutSucceeded(gomock.Any(), authapplication.MetricsOperationLogoutAll)

	result, err := fixture.LogoutAllSessions(ctx)

	if err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if result == nil || !result.LoggedOut {
		t.Fatalf("result=%#v", result)
	}
}

func TestAuthUseCaseLogoutAllSucceedsWhenRevocationProjectionFails(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")
	revocation := &authdomain.SessionRevocationResult{UserID: authTestUserID, TokenVersion: 3, ProjectionError: errors.New("projection failed")}

	fixture.sessions.EXPECT().RevokeAllUserSessions(gomock.Any(), authTestUserID).Return(revocation, nil)

	result, err := fixture.LogoutAllSessions(ctx)

	if err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if result == nil || !result.LoggedOut {
		t.Fatalf("result = %#v", result)
	}
}

func TestAuthUseCaseLogoutAllMapsIncrementUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	fixture.sessions.EXPECT().RevokeAllUserSessions(gomock.Any(), authTestUserID).Return(nil, identity.ErrUserNotFound)
	metrics.EXPECT().LogoutFailed(gomock.Any(), authapplication.MetricsOperationLogoutAll, authapplication.MetricsReasonSessionRevokeFailed)

	_, err := fixture.LogoutAllSessions(ctx)

	if !errors.Is(err, identity.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

type testAuthUseCases struct {
	LoginUseCase
	RefreshTokenUseCase
	ChangePasswordUseCase
	LogoutCurrentSessionUseCase
	LogoutAllSessionsUseCase
}

type authCommandFixture struct {
	testAuthUseCases
	credentials *MockVerifier
	tokens      *MockIssuer
	sessions    *MockLifecycle
}

func newAuthCommandFixture(t testing.TB, authCfg config.AuthConfig, metrics authapplication.Metrics) *authCommandFixture {
	t.Helper()
	return newAuthCommandFixtureWithController(gomock.NewController(t), authCfg, metrics)
}

func newAuthCommandFixtureWithController(ctrl *gomock.Controller, authCfg config.AuthConfig, metrics authapplication.Metrics) *authCommandFixture {
	credentials := NewMockVerifier(ctrl)
	tokens := NewMockIssuer(ctrl)
	sessions := NewMockLifecycle(ctrl)
	cfg := &config.Config{Auth: authCfg}
	return &authCommandFixture{
		testAuthUseCases: testAuthUseCases{
			LoginUseCase: NewLoginUseCase(LoginDeps{
				Credentials: credentials,
				Tokens:      tokens,
				Sessions:    sessions,
				Metrics:     metrics,
			}),
			RefreshTokenUseCase: NewRefreshTokenUseCase(RefreshTokenDeps{
				Tokens:   tokens,
				Sessions: sessions,
				Config:   cfg,
				Metrics:  metrics,
			}),
			ChangePasswordUseCase: NewChangePasswordUseCase(ChangePasswordDeps{
				Credentials: credentials,
				Tokens:      tokens,
				Sessions:    sessions,
			}),
			LogoutCurrentSessionUseCase: NewLogoutCurrentSessionUseCase(LogoutCurrentSessionDeps{
				Sessions: sessions,
				Metrics:  metrics,
			}),
			LogoutAllSessionsUseCase: NewLogoutAllSessionsUseCase(LogoutAllSessionsDeps{
				Sessions: sessions,
				Metrics:  metrics,
			}),
		},
		credentials: credentials,
		tokens:      tokens,
		sessions:    sessions,
	}
}

func defaultAuthConfig(rotation bool) config.AuthConfig {
	return config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: rotation, TokenVersionCacheTTL: time.Minute, MaxActiveSessionsPerUser: 5}
}

func normalCredential() *authdomain.UserCredential {
	return &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", Status: identity.UserStatusNormal, TokenVersion: 2}
}

func issuedTokenPair(accessToken string, refreshToken string, expiresIn int64, refreshTTL time.Duration) *authtokens.IssuedTokenPair {
	return &authtokens.IssuedTokenPair{
		Response:   &authtokens.TokenResult{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: commonauth.TokenTypeBearer, ExpiresIn: expiresIn},
		RefreshTTL: refreshTTL,
	}
}

func refreshClaims(sessionID string, tokenVersion int64) *commonauth.Claims {
	return &commonauth.Claims{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}

func passwordChangeClaims(sessionID string, tokenVersion int64) *commonauth.Claims {
	return &commonauth.Claims{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}

func testPasswordService(t testing.TB) *password.Service {
	t.Helper()
	service, err := password.NewService(password.Options{Concurrency: 1, QueueSize: 1})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service
}

func hashTestPassword(t testing.TB, plain string) (string, error) {
	t.Helper()
	return testPasswordService(t).HashContext(context.Background(), plain)
}

func verifyTestPassword(t testing.TB, plain, encodedHash string) (bool, error) {
	t.Helper()
	return testPasswordService(t).VerifyContext(context.Background(), plain, encodedHash)
}
