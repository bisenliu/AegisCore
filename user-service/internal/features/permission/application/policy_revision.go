package application

import "context"

// LatestPolicyRevisionSource 提供数据库中已提交的最新 RBAC policy revision。
type LatestPolicyRevisionSource interface {
	LatestPolicyRevision(ctx context.Context) (int64, error)
}
