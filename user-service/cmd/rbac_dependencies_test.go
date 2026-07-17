package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	commonresources "github.com/aegiscore/common/runtime/resources"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestRBACPostgresConfigUsesServiceResource(t *testing.T) {
	want := commonresources.PostgresConfig{Host: "db.internal", Port: 5432, Username: "aegiscore", DBName: "users"}
	cfg := &serviceconfig.Config{Resources: serviceconfig.ResourcesConfig{Postgres: commonresources.PostgresConfigs{
		"primary_db": want,
		"audit_db":   {Host: "audit.internal", Port: 5432, Username: "audit", DBName: "audit"},
	}}}

	got, err := rbacPostgresConfig(cfg)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestRBACPostgresConfigRejectsMissingUserDatabase(t *testing.T) {
	_, err := rbacPostgresConfig(&serviceconfig.Config{})
	require.ErrorContains(t, err, "resources.postgres.primary_db config not found")
}

func TestChainCleanupRunsSecondBeforeFirstAndJoinsErrors(t *testing.T) {
	firstErr := errors.New("first failed")
	secondErr := errors.New("second failed")
	order := make([]string, 0, 2)

	cleanup := chainCleanup(
		func() error {
			order = append(order, "first")
			return firstErr
		},
		func() error {
			order = append(order, "second")
			return secondErr
		},
	)

	err := cleanup()

	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
	require.Equal(t, []string{"second", "first"}, order)
}
