package permission

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/fx"
)

// Fx 选项

// permissionLifecycleOptions 只注册运行时 hook，便于测试和 graph 生成单独使用 WiringModule。
var permissionLifecycleOptions = fx.Options(
	fx.Invoke(
		registerRBACLifecycle,
	),
)

// Fx 参数：生命周期

// RegisterRBACLifecycleParams 汇集 RBAC 启停依赖，启动顺序由 registerRBACLifecycle 统一控制。
type RegisterRBACLifecycleParams struct {
	fx.In

	Lifecycle       fx.Lifecycle
	EngineLifecycle permissionPolicyEngineLifecycle `name:"permission_policy_engine_lifecycle"`
	Initializer     permissionPolicyInitializer     `name:"permission_policy_initializer"`
	Watcher         policyWatcherRunner             `name:"permission_policy_watcher_runner"`
}

// Provider：生命周期注册

// registerRBACLifecycle 依次启动 engine root、初始化策略并启动 watcher。
func registerRBACLifecycle(params RegisterRBACLifecycleParams) {
	var (
		runMu     sync.Mutex
		runCancel context.CancelFunc
	)
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
			runMu.Lock()
			runCancel = cancel
			runMu.Unlock()
			if err := params.EngineLifecycle.Start(runCtx); err != nil {
				cancel()
				return err
			}
			params.Initializer.InitializeFailClosed(ctx)
			if err := params.Watcher.Start(runCtx); err != nil {
				cancel()
				return errors.Join(err, params.Watcher.Stop(ctx), params.EngineLifecycle.Stop(ctx))
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			runMu.Lock()
			cancel := runCancel
			runCancel = nil
			runMu.Unlock()
			return stopRBACLifecycle(ctx, cancel, params.Watcher.Stop, params.EngineLifecycle.Stop)
		},
	})
}

// 生命周期辅助函数

// stopRBACLifecycle 取消运行 root 后按 watcher、engine 顺序停止并聚合全部错误。
func stopRBACLifecycle(ctx context.Context, cancelRoot context.CancelFunc, stopWatcher func(context.Context) error, stopEngine func(context.Context) error) error {
	if cancelRoot != nil {
		cancelRoot()
	}
	return errors.Join(
		stopWatcher(ctx),
		stopEngine(ctx),
	)
}
