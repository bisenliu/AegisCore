package casbin

import (
	"context"
)

// InitializeFailClosed 执行启动期初始 policy 加载，失败时保持 fail-closed。
// 初始 reload 失败不会阻断服务启动；Enforce 在 enforcer 或 userRoles 缺失时返回 deny，避免因授权组件未就绪而放行请求。
func (e *Engine) InitializeFailClosed(ctx context.Context) {
	_ = e.Reload(ctx)
}
