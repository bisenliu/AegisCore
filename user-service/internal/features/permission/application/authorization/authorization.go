package authorization

import (
	"context"

	"github.com/google/uuid"
)

// Authorizer 定义 HTTP 等入站边界消费的授权服务。
type Authorizer interface {
	Enforce(ctx context.Context, userID string, pathTemplate string, method string) (bool, error)
}

// Engine 定义授权服务依赖的内存策略执行能力。
type Engine interface {
	Enforce(ctx context.Context, userID uuid.UUID, pathTemplate string, method string) (bool, error)
}

type service struct {
	engine Engine
}

// NewAuthorizer 构造基于内存策略引擎的授权服务。
func NewAuthorizer(engine Engine) Authorizer {
	return &service{engine: engine}
}

// Enforce 校验用户标识并委托内存策略引擎执行授权判断。
func (s *service) Enforce(ctx context.Context, userID string, pathTemplate string, method string) (bool, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return false, nil
	}
	return s.engine.Enforce(ctx, parsedUserID, pathTemplate, method)
}
