package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aegiscore/common/runtime/datastore"
	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

const bootstrapAdvisoryLockKey int64 = 4702111234474988097

// BootstrapStore 使用 PostgreSQL 事务完成一次性超级管理员引导。
type BootstrapStore struct {
	db *sql.DB
}

var _ rolebootstrap.BootstrapStore = (*BootstrapStore)(nil)

// NewBootstrapStore 构造 bootstrap store。
func NewBootstrapStore(db *sql.DB) *BootstrapStore {
	return &BootstrapStore{db: db}
}

// BootstrapSuperAdmin 创建固定 bootstrap 用户并绑定内置超级管理员角色。
func (s *BootstrapStore) BootstrapSuperAdmin(ctx context.Context, input rolebootstrap.BootstrapSuperAdminInput) (*rolebootstrap.BootstrapSuperAdminResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("bootstrap postgres store requires database")
	}
	tx, finish, err := datastore.BeginTransaction(ctx, sqlTxStarter{db: s.db})
	if err != nil {
		return nil, fmt.Errorf("begin bootstrap super admin: %w", err)
	}
	defer func() { _ = finish.RollbackUnlessCommitted() }()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, bootstrapAdvisoryLockKey); err != nil {
		return nil, finish.Fail(fmt.Errorf("lock bootstrap super admin: %w", err))
	}

	roleInternalID, err := queryBootstrapRole(ctx, tx, input)
	if err != nil {
		return nil, finish.Fail(err)
	}
	if exists, err := userIDExists(ctx, tx, input.UserID.String()); err != nil {
		return nil, finish.Fail(err)
	} else if exists {
		return nil, finish.Fail(rolebootstrap.ErrSuperAdminAlreadyBootstrapped)
	}
	if exists, err := usernameExists(ctx, tx, input.Username); err != nil {
		return nil, finish.Fail(err)
	} else if exists {
		return nil, finish.Fail(rolebootstrap.ErrBootstrapUsernameAlreadyExists)
	}

	now := time.Now().UnixMilli()
	userInternalID, err := insertBootstrapUser(ctx, tx, input, now)
	if err != nil {
		return nil, finish.Fail(err)
	}
	if err := insertBootstrapUserRole(ctx, tx, userInternalID, roleInternalID, now); err != nil {
		return nil, finish.Fail(err)
	}
	if err := finish.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit bootstrap super admin: %w", err)
	}
	return &rolebootstrap.BootstrapSuperAdminResult{UserID: input.UserID, RoleID: input.RoleID, Username: input.Username, Nickname: input.Nickname}, nil
}

func queryBootstrapRole(ctx context.Context, tx *sql.Tx, input rolebootstrap.BootstrapSuperAdminInput) (int64, error) {
	var internalID int64
	var isSystem bool
	var active bool
	err := tx.QueryRowContext(ctx, `SELECT id, is_system, active FROM roles WHERE role_id = $1`, input.RoleID.String()).Scan(&internalID, &isSystem, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, roledomain.ErrRoleNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("query bootstrap super admin role %s: %w", input.RoleID.String(), err)
	}
	if !isSystem {
		return 0, roledomain.ErrSystemRoleProtected
	}
	if !active {
		return 0, roledomain.ErrRoleInactive
	}
	return internalID, nil
}

func userIDExists(ctx context.Context, tx *sql.Tx, userID string) (bool, error) {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE user_id = $1)`, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query bootstrap user id %s: %w", userID, err)
	}
	return exists, nil
}

func usernameExists(ctx context.Context, tx *sql.Tx, username string) (bool, error) {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists); err != nil {
		return false, fmt.Errorf("query bootstrap username %s: %w", username, err)
	}
	return exists, nil
}

func insertBootstrapUser(ctx context.Context, tx *sql.Tx, input rolebootstrap.BootstrapSuperAdminInput, now int64) (int64, error) {
	var internalID int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO users (user_id, nickname, username, password_hash, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id
	`, input.UserID.String(), input.Nickname, input.Username, input.PasswordHash, int64(input.Status), now).Scan(&internalID)
	if err == nil {
		return internalID, nil
	}
	return 0, mapBootstrapConstraintError(err, "create bootstrap user")
}

func insertBootstrapUserRole(ctx context.Context, tx *sql.Tx, userInternalID int64, roleInternalID int64, now int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, role_id, created_at)
		VALUES ($1, $2, $3)
	`, userInternalID, roleInternalID, now)
	if err == nil {
		return nil
	}
	return mapBootstrapConstraintError(err, "create bootstrap user role")
}

func mapBootstrapConstraintError(err error, operation string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_user_id_key", "userrole_user_id_role_id":
			return rolebootstrap.ErrSuperAdminAlreadyBootstrapped
		case "users_username_key":
			return rolebootstrap.ErrBootstrapUsernameAlreadyExists
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
