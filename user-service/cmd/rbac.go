package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const (
	defaultCreateSuperAdminUsername    = "admin"
	defaultCreateSuperAdminNickname    = "Admin"
	defaultCreateSuperAdminPasswordEnv = "ADMIN_PASSWORD"
)

type rbacSeedOptions struct {
	reactivateSystem   bool
	syncSystemBindings bool
}

type rbacCreateSuperAdminOptions struct {
	// password 由 normalizeCreateSuperAdminOptions 从 passwordEnv 读取，避免命令行参数泄露明文。
	username    string
	nickname    string
	password    string
	passwordEnv string
	// resetPassword 只在用户已存在时更新密码并恢复 normal 状态；默认不会覆盖既有管理员凭据。
	resetPassword bool
}

type rbacSeedRunner func(context.Context, string, rbacSeedOptions) error

type rbacAssignSuperAdminRunner func(context.Context, string, uuid.UUID) error

type rbacCreateSuperAdminRunner func(context.Context, string, rbacCreateSuperAdminOptions) error

func newRBACCommand(seedRunner rbacSeedRunner, assignRunner rbacAssignSuperAdminRunner, createRunner rbacCreateSuperAdminRunner) *cobra.Command {
	var configPath string
	var seedOpts rbacSeedOptions
	var superAdminUserID string
	createOpts := rbacCreateSuperAdminOptions{
		username:    defaultCreateSuperAdminUsername,
		nickname:    defaultCreateSuperAdminNickname,
		passwordEnv: defaultCreateSuperAdminPasswordEnv,
	}

	cmd := &cobra.Command{
		Use:   "rbac",
		Short: "Manage RBAC seed data and bootstrap bindings",
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "./configs/config.yaml", "path to YAML configuration file")

	seed := &cobra.Command{
		Use:   "seed",
		Short: "Seed default RBAC system roles, permissions, and bindings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return seedRunner(cmd.Context(), configPath, seedOpts)
		},
	}
	seed.Flags().BoolVar(&seedOpts.reactivateSystem, "reactivate-system", false, "reactivate catalog-managed system roles and permissions")
	seed.Flags().BoolVar(&seedOpts.syncSystemBindings, "sync-system-bindings", false, "synchronize catalog-managed system role permission bindings exactly")
	cmd.AddCommand(seed)

	assignSuperAdmin := &cobra.Command{
		Use:   "assign-super-admin --user-id <uuid>",
		Short: "Assign the built-in super admin role to a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			userID, err := uuid.Parse(superAdminUserID)
			if err != nil {
				return fmt.Errorf("invalid --user-id: %w", err)
			}
			return assignRunner(cmd.Context(), configPath, userID)
		},
	}
	assignSuperAdmin.Flags().StringVar(&superAdminUserID, "user-id", "", "user UUID to receive the built-in super admin role")
	_ = assignSuperAdmin.MarkFlagRequired("user-id")
	cmd.AddCommand(assignSuperAdmin)

	createSuperAdmin := &cobra.Command{
		Use:   "create-super-admin",
		Short: "Create the default admin user and assign the built-in super admin role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return createRunner(cmd.Context(), configPath, createOpts)
		},
	}
	createSuperAdmin.Flags().StringVar(&createOpts.username, "username", defaultCreateSuperAdminUsername, "admin username to create or bind")
	createSuperAdmin.Flags().StringVar(&createOpts.nickname, "nickname", defaultCreateSuperAdminNickname, "admin display nickname")
	createSuperAdmin.Flags().StringVar(&createOpts.passwordEnv, "password-env", defaultCreateSuperAdminPasswordEnv, "environment variable containing the admin password")
	createSuperAdmin.Flags().BoolVar(&createOpts.resetPassword, "reset-password", false, "reset password when the admin user already exists")
	cmd.AddCommand(createSuperAdmin)

	return cmd
}
