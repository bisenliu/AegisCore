package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandSurface(t *testing.T) {
	root := newRootCommand(testRootCommandDependencies(t))
	require.Equal(t, "aegiscore-user-services", root.Use)

	var serve *cobra.Command
	var rbac *cobra.Command
	var fxGraph *cobra.Command
	var healthcheck *cobra.Command
	for _, cmd := range root.Commands() {
		switch cmd.Use {
		case "serve":
			serve = cmd
		case "rbac":
			rbac = cmd
		case "fxgraph":
			fxGraph = cmd
		case "healthcheck":
			healthcheck = cmd
		}
	}
	require.NotNil(t, serve)

	flag := serve.Flags().Lookup("config")
	require.NotNil(t, flag)
	assert.Equal(t, "./configs/config.yaml", flag.DefValue)
	require.NotNil(t, rbac)
	flag = rbac.PersistentFlags().Lookup("config")
	require.NotNil(t, flag)
	assert.Equal(t, "./configs/config.yaml", flag.DefValue)
	assert.NotNil(t, findSubcommand(rbac, "seed"))
	assert.NotNil(t, findSubcommand(rbac, "assign-super-admin"))
	assert.NotNil(t, findSubcommand(rbac, "create-super-admin"))
	require.NotNil(t, fxGraph)
	flag = fxGraph.Flags().Lookup("config")
	require.NotNil(t, flag)
	assert.Equal(t, "./configs/config.yaml", flag.DefValue)
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
