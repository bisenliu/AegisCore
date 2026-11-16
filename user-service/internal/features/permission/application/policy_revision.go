package application

import (
	"context"
	"uuid"
)

const DefaultUserRoleRevisionBatchSize = 100

// LatestPolicyRevisionSource 提供数据库中已提交的最新 RBAC policy revision。
type LatestPolicyRevisionSource interface {
	LatestPolicyRevision(ctx context.Context) (int64, error)
}

// UserRoleRevisionChange 是一条已提交 user-role revision 及其受影响用户。
type UserRoleRevisionChange struct {
	Revision int64
	UserID   uuid.UUID
}

// UserRoleRevisionBatch 是高于调用方水位的有界 user-role revision 查询结果。
type UserRoleRevisionBatch struct {
	Changes        []UserRoleRevisionChange
	LatestRevision int64
	Truncated      bool
}

// UserRoleRevisionSource 按 revision 水位读取已提交的用户角色变更。
type UserRoleRevisionSource interface {
	UserRoleRevisionsAfter(ctx context.Context, afterRevision int64, limit int) (UserRoleRevisionBatch, error)
}
