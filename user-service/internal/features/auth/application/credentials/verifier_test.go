package credentials

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/aegiscore/common/security/password"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var verifierTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestVerifierAcceptsMustChangePasswordUser(t *testing.T) {
	passwordHash, err := hashTestPassword(t, "secret")
	require.NoError(t, err,
		"Hash: %v", err)

	store := NewMockUserCredentialStore(gomock.NewController(t))
	store.EXPECT().GetByUsername(gomock.Any(), "alice").Return(&authdomain.UserCredential{UserID: verifierTestUserID, Username: "alice", PasswordHash: passwordHash, Status: identity.UserStatusMustChangePassword, TokenVersion: 2}, nil)
	verifier := NewVerifier(store, testPasswordService(t))

	user, err := verifier.VerifyPassword(context.Background(), "alice", "secret")
	require.NoError(t, err,
		"VerifyPassword: %v", err)
	require.True(t, user.RequiresPasswordChange(),
		"user status = %d, want must change password", user.Status)

}

func TestVerifierRejectsDisabledUser(t *testing.T) {
	passwordHash, err := hashTestPassword(t, "secret")
	require.NoError(t, err,
		"Hash: %v", err)

	store := NewMockUserCredentialStore(gomock.NewController(t))
	store.EXPECT().GetByUsername(gomock.Any(), "alice").Return(&authdomain.UserCredential{UserID: verifierTestUserID, Username: "alice", PasswordHash: passwordHash, Status: identity.UserStatusDisabled, TokenVersion: 2}, nil)
	verifier := NewVerifier(store, testPasswordService(t))

	_, err = verifier.VerifyPassword(context.Background(), "alice", "secret")
	require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
		"err = %v, want ErrInvalidCredentials", err)

}

func TestVerifierChangePasswordUpdatesCredentials(t *testing.T) {
	oldHash, err := hashTestPassword(t, "old-secret")
	require.NoError(t, err,
		"Hash: %v", err)

	store := NewMockUserCredentialStore(gomock.NewController(t))
	var updatedInput authdomain.UpdateCredentialsInput
	store.EXPECT().GetCredentialByUserID(gomock.Any(), verifierTestUserID).Return(&authdomain.UserCredential{UserID: verifierTestUserID, PasswordHash: oldHash, Status: identity.UserStatusMustChangePassword, TokenVersion: 2}, nil)
	store.EXPECT().UpdateCredentials(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
		updatedInput = input
		return int64(3), nil
	})
	verifier := NewVerifier(store, testPasswordService(t))

	result, err := verifier.ChangePassword(context.Background(), verifierTestUserID, 2, "new-secret")
	require.NoError(t, err,
		"ChangePassword: %v", err)
	require.False(t, result.UserID != verifierTestUserID || result.TokenVersion != 3,
		"result = %#v", result)
	require.False(t, updatedInput.UserID != verifierTestUserID || updatedInput.Status != identity.UserStatusNormal,
		"updated input = %#v", updatedInput)
	require.False(t, updatedInput.ExpectedStatus == nil || *updatedInput.ExpectedStatus != identity.UserStatusMustChangePassword || updatedInput.ExpectedTokenVersion == nil || *updatedInput.ExpectedTokenVersion != 2,
		"updated input expected guards = %#v", updatedInput)

}

func testPasswordService(t testing.TB) *password.Service {
	t.Helper()
	service, err := password.NewService(password.Options{Concurrency: 1, QueueSize: 1})
	require.NoError(t, err,
		"NewService: %v", err)

	return service
}

func hashTestPassword(t testing.TB, plain string) (string, error) {
	t.Helper()
	return testPasswordService(t).HashContext(context.Background(), plain)
}
