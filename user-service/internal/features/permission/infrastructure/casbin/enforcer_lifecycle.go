package casbin

import (
	"context"
	"errors"
)

// Start 建立 engine lifecycle root；reload flight 必须从该 root 派生。
func (e *Engine) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("casbin engine lifecycle context is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lifecycleDone {
		return errors.New("casbin engine lifecycle is stopped")
	}
	if e.lifecycleCancel != nil {
		e.lifecycleCancel()
	}
	e.lifecycleCtx, e.lifecycleCancel = context.WithCancel(ctx)
	e.lifecycleStarted = true
	return nil
}

// Stop 取消 engine lifecycle root；ctx 只表示本次停止调用边界。
func (e *Engine) Stop(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lifecycleDone = true
	e.lifecycleStarted = false
	if e.lifecycleCancel != nil {
		e.lifecycleCancel()
	}
	return nil
}

// InitializeFailClosed 执行启动期初始 policy 加载，失败时保持 fail-closed。
// 初始 reload 失败不会阻断服务启动；Enforce 在 policy 投影未就绪时返回 deny，避免因授权组件未就绪而放行请求。
func (e *Engine) InitializeFailClosed(ctx context.Context) {
	_, _ = e.ReloadToRevision(ctx, 0)
}
