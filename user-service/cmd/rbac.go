package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const defaultBootstrapSuperAdminPasswordEnv = "ADMIN_BOOTSTRAP_PASSWORD"

type rbacSeedOptions struct {
	reactivateSystem   bool
	syncSystemBindings bool
}

type rbacBootstrapSuperAdminOptions struct {
	username    string
	nickname    string
	passwordEnv string
}

type rbacSeedRunner func(context.Context, string, rbacSeedOptions) error

type rbacBootstrapSuperAdminRunner func(context.Context, string, rbacBootstrapSuperAdminOptions) error

func newRBACCommand(seedRunner rbacSeedRunner, bootstrapRunner rbacBootstrapSuperAdminRunner) *cobra.Command {
	var configPath string
	var seedOpts rbacSeedOptions
	bootstrapOpts := rbacBootstrapSuperAdminOptions{
		passwordEnv: defaultBootstrapSuperAdminPasswordEnv,
	}

	cmd := &cobra.Command{
		Use:   "rbac",
		Short: "Manage RBAC seed data and bootstrap super admin",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("rbac subcommand is required")
		},
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "./configs/config.yaml", "path to YAML configuration file")

	seed := &cobra.Command{
		Use:   "seed",
		Short: "Seed default RBAC system roles, permissions, and bindings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return seedRunner(cmd.Context(), configPath, seedOpts)
		},
	}
	seed.Flags().BoolVar(&seedOpts.reactivateSystem, "reactivate-system", false, "reactivate catalog-managed system roles")
	seed.Flags().BoolVar(&seedOpts.syncSystemBindings, "sync-system-bindings", false, "synchronize catalog-managed system role permission bindings exactly")
	cmd.AddCommand(seed)

	bootstrapSuperAdmin := &cobra.Command{
		Use:   "bootstrap-super-admin",
		Short: "Bootstrap the initial super admin user once",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return bootstrapRunner(cmd.Context(), configPath, bootstrapOpts)
		},
	}
	bootstrapSuperAdmin.Flags().StringVar(&bootstrapOpts.username, "username", "", "initial admin username")
	bootstrapSuperAdmin.Flags().StringVar(&bootstrapOpts.nickname, "nickname", "", "initial admin display nickname")
	bootstrapSuperAdmin.Flags().StringVar(&bootstrapOpts.passwordEnv, "password-env", defaultBootstrapSuperAdminPasswordEnv, "environment variable containing the bootstrap password")
	_ = bootstrapSuperAdmin.MarkFlagRequired("username")
	cmd.AddCommand(bootstrapSuperAdmin)

	return cmd
}
