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

	Lifecycle fx.Lifecycle
	Runtime   *PermissionRuntime
}

// Provider：生命周期注册

// registerRBACLifecycle 依次启动 engine root、初始化策略、启动 watcher 和 outbox dispatcher。
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
			if err := params.Runtime.EngineLifecycle.Start(runCtx); err != nil {
				cancel()
				return err
			}
			params.Runtime.Initializer.InitializeFailClosed(ctx)
			if err := params.Runtime.Watcher.Start(runCtx); err != nil {
				cancel()
				return errors.Join(err, params.Runtime.Watcher.Stop(ctx), params.Runtime.EngineLifecycle.Stop(ctx))
			}
			if err := params.Runtime.Dispatcher.Start(runCtx); err != nil {
				cancel()
				return errors.Join(err, params.Runtime.Watcher.Stop(ctx), params.Runtime.EngineLifecycle.Stop(ctx))
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			runMu.Lock()
			cancel := runCancel
			runCancel = nil
			runMu.Unlock()
			return stopRBACLifecycle(ctx, cancel, params.Runtime.Dispatcher.Stop, params.Runtime.Watcher.Stop, params.Runtime.EngineLifecycle.Stop)
		},
	})
}

// 生命周期辅助函数

// stopRBACLifecycle 按 dispatcher、watcher 顺序停止并聚合全部错误。
func stopRBACLifecycle(ctx context.Context, cancelRoot context.CancelFunc, stopDispatcher func(context.Context) error, stopWatcher func(context.Context) error, stopEngine func(context.Context) error) error {
	dispatcherErr := stopDispatcher(ctx)
	if cancelRoot != nil {
		cancelRoot()
	}
	return errors.Join(
		dispatcherErr,
		stopWatcher(ctx),
		stopEngine(ctx),
	)
}
