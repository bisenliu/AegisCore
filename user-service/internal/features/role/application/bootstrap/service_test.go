package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/user-service/internal/shared/identity"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestServiceBootstrapSuperAdmin(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "  long secret  ")
	store := &recordingStore{}
	hasher := &recordingHasher{hash: "bcrypt-hash"}
	service := NewService(store, hasher)

	result, err := service.BootstrapSuperAdmin(context.Background(), Command{Username: " Initial-Admin ", Nickname: " ", PasswordEnv: "ADMIN_SECRET"})

	require.NoError(t, err)
	require.Equal(t, uuid.MustParse(rbacbaseline.BootstrapSuperAdminUserID), result.UserID)
	require.Equal(t, uuid.MustParse(rbacbaseline.SuperAdminRoleID), result.RoleID)
	require.Equal(t, "initial-admin", result.Username)
	require.Equal(t, "initial-admin", result.Nickname)
	require.Equal(t, "  long secret  ", hasher.plain)
	require.Equal(t, uuid.MustParse(rbacbaseline.BootstrapSuperAdminUserID), store.input.UserID)
	require.Equal(t, uuid.MustParse(rbacbaseline.SuperAdminRoleID), store.input.RoleID)
	require.Equal(t, identity.UserStatusMustChangePassword, store.input.Status)
	require.Equal(t, "bcrypt-hash", store.input.PasswordHash)
}

func TestServiceBootstrapSuperAdminNickname(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "long-password")
	store := &recordingStore{}
	service := NewService(store, &recordingHasher{hash: "hash"})

	_, err := service.BootstrapSuperAdmin(context.Background(), Command{Username: "Admin", Nickname: " Initial Administrator ", PasswordEnv: "ADMIN_SECRET"})

	require.NoError(t, err)
	require.Equal(t, "Initial Administrator", store.input.Nickname)
}

func TestServiceBootstrapSuperAdminValidation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		setup      func(t *testing.T)
		cmd        Command
		wantErr    string
		wantNoHash bool
	}{
		{name: "requires username", setup: func(t *testing.T) { t.Setenv("ADMIN_SECRET", "long-password") }, cmd: Command{Username: " ", PasswordEnv: "ADMIN_SECRET"}, wantErr: "bootstrap username is required", wantNoHash: true},
		{name: "requires password env", setup: func(*testing.T) {}, cmd: Command{Username: "admin", PasswordEnv: "MISSING_SECRET"}, wantErr: "MISSING_SECRET environment variable is required", wantNoHash: true},
		{name: "rejects short password", setup: func(t *testing.T) { t.Setenv("ADMIN_SECRET", "short") }, cmd: Command{Username: "admin", PasswordEnv: "ADMIN_SECRET"}, wantErr: "at least 12 bytes", wantNoHash: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			hasher := &recordingHasher{hash: "hash"}
			_, err := NewService(&recordingStore{}, hasher).BootstrapSuperAdmin(context.Background(), tc.cmd)
			require.ErrorContains(t, err, tc.wantErr)
			require.ErrorIs(t, err, ErrBootstrapInvalidInput)
			if tc.wantNoHash {
				require.Empty(t, hasher.plain)
			}
		})
	}
}

func TestServiceBootstrapSuperAdminPropagatesHashAndStoreErrors(t *testing.T) {
	t.Run("hash error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "long-password")
		hashErr := errors.New("hash failed")
		_, err := NewService(&recordingStore{}, &recordingHasher{err: hashErr}).BootstrapSuperAdmin(context.Background(), Command{Username: "admin", PasswordEnv: "ADMIN_SECRET"})
		require.ErrorIs(t, err, hashErr)
	})

	t.Run("store error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "long-password")
		storeErr := errors.New("store failed")
		_, err := NewService(&recordingStore{err: storeErr}, &recordingHasher{hash: "hash"}).BootstrapSuperAdmin(context.Background(), Command{Username: "admin", PasswordEnv: "ADMIN_SECRET"})
		require.ErrorIs(t, err, storeErr)
	})
}

type recordingHasher struct {
	plain string
	hash  string
	err   error
}

func (h *recordingHasher) HashContext(_ context.Context, plain string) (string, error) {
	h.plain = plain
	if h.err != nil {
		return "", h.err
	}
	return h.hash, nil
}

type recordingStore struct {
	input BootstrapSuperAdminInput
	err   error
}

func (s *recordingStore) BootstrapSuperAdmin(_ context.Context, input BootstrapSuperAdminInput) (*BootstrapSuperAdminResult, error) {
	s.input = input
	if s.err != nil {
		return nil, s.err
	}
	return &BootstrapSuperAdminResult{UserID: input.UserID, RoleID: input.RoleID, Username: input.Username, Nickname: input.Nickname}, nil
}
