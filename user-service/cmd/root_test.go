package main

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestRootCommandSurface(t *testing.T) {
	root := newRootCommand(testRootCommandDependencies(t))
	require.Equal(t, "aegiscore-user-service", root.Use)

	var serve *cobra.Command
	var rbac *cobra.Command
	var configCmd *cobra.Command
	var fxGraph *cobra.Command
	var healthcheck *cobra.Command
	for _, cmd := range root.Commands() {
		switch cmd.Use {
		case "serve":
			serve = cmd
		case "rbac":
			rbac = cmd
		case "config":
			configCmd = cmd
		case "fxgraph":
			fxGraph = cmd
		case "healthcheck":
			healthcheck = cmd
		}
	}
	require.NotNil(t, serve)

	flag := serve.Flags().Lookup("config")
	require.Nil(t, flag)
	require.NotNil(t, rbac)
	flag = rbac.PersistentFlags().Lookup("config")
	require.Nil(t, flag)
	assert.NotNil(t, findSubcommand(rbac, "seed"))
	assert.NotNil(t, findSubcommand(rbac, "bootstrap-super-admin"))
	assert.Nil(t, findSubcommand(rbac, "assign-super-admin"))
	assert.Nil(t, findSubcommand(rbac, "create-super-admin"))
	require.NotNil(t, configCmd)
	assert.NotNil(t, findSubcommand(configCmd, "validate"))
	assert.NotNil(t, findSubcommand(configCmd, "render"))
	assert.NotNil(t, findSubcommand(configCmd, "sources"))
	require.NotNil(t, fxGraph)
	flag = fxGraph.Flags().Lookup("config")
	require.Nil(t, flag)
	flag = fxGraph.Flags().Lookup("output")
	require.NotNil(t, flag)
	assert.Equal(t, defaultFxGraphOutputPath, flag.DefValue)
	require.NotNil(t, healthcheck)
	flag = healthcheck.Flags().Lookup("url")
	require.NotNil(t, flag)
	assert.Equal(t, defaultHealthcheckURL, flag.DefValue)
	flag = healthcheck.Flags().Lookup("timeout")
	require.NotNil(t, flag)
	assert.Equal(t, defaultHealthcheckTimeout.String(), flag.DefValue)
}

func TestWithConfigLoadTimeoutCancelsSlowLoader(t *testing.T) {
	started := make(chan struct{})
	wrapped := withConfigLoadTimeout(func(ctx context.Context) (*serviceconfig.LoadResult, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}, 10*time.Millisecond)

	_, err := wrapped(context.Background())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	<-started
}

func TestWithConfigLoadTimeoutCanBeDisabled(t *testing.T) {
	called := false
	wrapped := withConfigLoadTimeout(func(ctx context.Context) (*serviceconfig.LoadResult, error) {
		called = true
		_, ok := ctx.Deadline()
		require.False(t, ok)
		return nil, nil
	}, 0)

	_, err := wrapped(context.Background())
	require.NoError(t, err)
	require.True(t, called)
}
