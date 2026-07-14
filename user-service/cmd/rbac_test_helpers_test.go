package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
)

func rbacCommandRunnersWithFactory(factory rbacSeedDependencyFactory) rootCommandDependencies {
	return rootCommandDependencies{
		seedRunner:             newRBACSeedRunner(factory),
		assignSuperAdminRunner: newRBACAssignSuperAdminRunner(factory),
		createSuperAdminRunner: newRBACCreateSuperAdminRunner(factory),
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

func newRBACSeedServiceMock(t testing.TB, seed func(context.Context, roleseed.SeedOptions) (roleseed.SeedResult, error), assign func(context.Context, uuid.UUID) (roleseed.AssignSuperAdminResult, error)) *MockrbacSeedService {
	t.Helper()
	service := NewMockrbacSeedService(gomock.NewController(t))
	if seed != nil {
		service.EXPECT().Seed(gomock.Any(), gomock.Any()).DoAndReturn(seed)
	}
	if assign != nil {
		service.EXPECT().AssignSuperAdmin(gomock.Any(), gomock.Any()).DoAndReturn(assign)
	}
	return service
}

func newCreateUserServiceMock(t testing.TB, create func(context.Context, usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error)) *MockCreateUserService {
	t.Helper()
	service := NewMockCreateUserService(gomock.NewController(t))
	service.EXPECT().CreateUser(gomock.Any(), gomock.Any()).DoAndReturn(create)
	return service
}

func newRBACCredentialStoreMock(t testing.TB, getByUsername func(context.Context, string) (*authdomain.UserCredential, error), updateCredentials func(context.Context, authdomain.UpdateCredentialsInput) (int64, error)) *MockrbacCredentialStore {
	t.Helper()
	store := NewMockrbacCredentialStore(gomock.NewController(t))
	if getByUsername != nil {
		store.EXPECT().GetByUsername(gomock.Any(), gomock.Any()).DoAndReturn(getByUsername)
	}
	if updateCredentials != nil {
		store.EXPECT().UpdateCredentials(gomock.Any(), gomock.Any()).DoAndReturn(updateCredentials)
	}
	return store
}

func newRBACPasswordHasherMock(t testing.TB, hash func(context.Context, string) (string, error)) *MockrbacPasswordHasher {
	t.Helper()
	hasher := NewMockrbacPasswordHasher(gomock.NewController(t))
	hasher.EXPECT().HashContext(gomock.Any(), gomock.Any()).DoAndReturn(hash)
	return hasher
}
