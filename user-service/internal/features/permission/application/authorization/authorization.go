package authorization

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

// ErrInvalidSubjectUserID 表示授权 subject 中的用户 ID 不是合法 UUID。
var ErrInvalidSubjectUserID = errors.New("authorization subject user id is invalid")

// Authorizer 定义 HTTP 等入站边界消费的授权服务。
type Authorizer interface {
	Enforce(ctx context.Context, userID string, pathTemplate string, method string) (bool, error)
}

// PolicyHealth 暴露 RBAC policy engine 的只读健康状态。
type PolicyHealth interface {
	ProjectionStatus() permissionapplication.PolicyProjectionStatus
}

// Engine 定义授权服务依赖的内存策略执行能力。
type Engine interface {
	Enforce(ctx context.Context, userID uuid.UUID, pathTemplate string, method string) (bool, error)
}

type service struct {
	engine  Engine
	metrics permissionapplication.Metrics
}

// NewAuthorizer 构造基于内存策略引擎的授权服务。
func NewAuthorizer(engine Engine, metrics permissionapplication.Metrics) Authorizer {
	if metrics == nil {
		metrics = permissionapplication.NopMetrics()
	}
	return &service{engine: engine, metrics: metrics}
}

// Enforce 校验用户标识并委托内存策略引擎执行授权判断。
func (s *service) Enforce(ctx context.Context, userID string, pathTemplate string, method string) (bool, error) {
	startedAt := time.Now()
	result := permissionapplication.MetricsEnforceResultError
	defer func() {
		s.metrics.EnforceObserved(ctx, result, method, pathTemplate, time.Since(startedAt))
	}()

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrInvalidSubjectUserID, err)
	}
	allowed, err := s.engine.Enforce(ctx, parsedUserID, pathTemplate, method)
	if err != nil {
		return false, err
	}
	if allowed {
		result = permissionapplication.MetricsEnforceResultAllow
		return true, nil
	}
	result = permissionapplication.MetricsEnforceResultDeny
	return false, nil
}
