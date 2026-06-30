package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/aegiscore/common/security/password"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var verifierTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestVerifierAcceptsMustChangePasswordUser(t *testing.T) {
	passwordHash, err := hashTestPassword(t, "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	verifier := NewVerifier(&credentialStoreStub{userByUsername: &authdomain.UserCredential{UserID: verifierTestUserID, Username: "alice", PasswordHash: passwordHash, Status: identity.UserStatusMustChangePassword, TokenVersion: 2}}, testPasswordService(t))

	user, err := verifier.VerifyPassword(context.Background(), "alice", "secret")

	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !user.RequiresPasswordChange() {
		t.Fatalf("user status = %d, want must change password", user.Status)
	}
}

func TestVerifierRejectsDisabledUser(t *testing.T) {
	passwordHash, err := hashTestPassword(t, "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	verifier := NewVerifier(&credentialStoreStub{userByUsername: &authdomain.UserCredential{UserID: verifierTestUserID, Username: "alice", PasswordHash: passwordHash, Status: identity.UserStatusDisabled, TokenVersion: 2}}, testPasswordService(t))

	_, err = verifier.VerifyPassword(context.Background(), "alice", "secret")

	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestVerifierChangePasswordUpdatesCredentials(t *testing.T) {
	oldHash, err := hashTestPassword(t, "old-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	store := &credentialStoreStub{userByID: &authdomain.UserCredential{UserID: verifierTestUserID, PasswordHash: oldHash, Status: identity.UserStatusMustChangePassword, TokenVersion: 2}, newVersion: 3}
	verifier := NewVerifier(store, testPasswordService(t))

	result, err := verifier.ChangePassword(context.Background(), verifierTestUserID, "new-secret")

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if result.UserID != verifierTestUserID || result.TokenVersion != 3 {
		t.Fatalf("result = %#v", result)
	}
	if store.updatedInput.UserID != verifierTestUserID || store.updatedInput.Status != identity.UserStatusNormal {
		t.Fatalf("updated input = %#v", store.updatedInput)
	}
}

type credentialStoreStub struct {
	userByUsername *authdomain.UserCredential
	userByID       *authdomain.UserCredential
	newVersion     int64
	updatedInput   authdomain.UpdateCredentialsInput
}

func (s *credentialStoreStub) GetByUsername(context.Context, string) (*authdomain.UserCredential, error) {
	if s.userByUsername == nil {
		return nil, identity.ErrUserNotFound
	}
	return s.userByUsername, nil
}

func (s *credentialStoreStub) GetCredentialByUserID(context.Context, uuid.UUID) (*authdomain.UserCredential, error) {
	if s.userByID == nil {
		return nil, identity.ErrUserNotFound
	}
	return s.userByID, nil
}

func (s *credentialStoreStub) UpdateCredentials(_ context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
	s.updatedInput = input
	return s.newVersion, nil
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
