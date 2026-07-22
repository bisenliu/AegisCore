package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
)

func rbacCommandRunnersWithFactory(factory rbacSeedDependencyFactory) rootCommandDependencies {
	return rootCommandDependencies{
		seedRunner:                newRBACSeedRunner(factory),
		bootstrapSuperAdminRunner: newRBACBootstrapSuperAdminRunner(factory),
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer

	runErr := fn()
	closeErr := writer.Close()
	os.Stdout = original
	require.NoError(t, closeErr)

	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return string(output), runErr
}

func newRBACSeedServiceMock(t testing.TB, seed func(context.Context, roleseed.SeedOptions) (roleseed.SeedResult, error)) *MockrbacSeedService {
	t.Helper()
	service := NewMockrbacSeedService(gomock.NewController(t))
	if seed != nil {
		service.EXPECT().Seed(gomock.Any(), gomock.Any()).DoAndReturn(seed)
	}
	return service
}

type testBootstrapStore struct {
	input rolebootstrap.BootstrapSuperAdminInput
	err   error
}

func (s *testBootstrapStore) BootstrapSuperAdmin(_ context.Context, input rolebootstrap.BootstrapSuperAdminInput) (*rolebootstrap.BootstrapSuperAdminResult, error) {
	s.input = input
	if s.err != nil {
		return nil, s.err
	}
	return &rolebootstrap.BootstrapSuperAdminResult{UserID: input.UserID, RoleID: input.RoleID, Username: input.Username, Nickname: input.Nickname}, nil
}

type testBootstrapHasher struct {
	plain string
	err   error
}

func (h *testBootstrapHasher) HashContext(_ context.Context, plain string) (string, error) {
	h.plain = plain
	if h.err != nil {
		return "", h.err
	}
	return "hashed-password", nil
}
