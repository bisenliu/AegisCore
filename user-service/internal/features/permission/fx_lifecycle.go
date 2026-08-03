package permission

import (
	"context"
	"errors"

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

// registerRBACLifecycle 依次启动 resolver、策略初始化、watcher 和 outbox dispatcher。
func registerRBACLifecycle(params RegisterRBACLifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := params.Runtime.UserRoles.Start(ctx); err != nil {
				return err
			}
			params.Runtime.Initializer.InitializeFailClosed(ctx)
			params.Runtime.Watcher.Start()
			if err := params.Runtime.Dispatcher.Start(); err != nil {
				return errors.Join(err, params.Runtime.Watcher.Stop(ctx), params.Runtime.UserRoles.Close())
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return stopRBACLifecycle(ctx, params.Runtime.Dispatcher.Stop, params.Runtime.Watcher.Stop, params.Runtime.UserRoles)
		},
	})
}

// 生命周期辅助函数

// stopRBACLifecycle 按 dispatcher、watcher、resolver 顺序停止并聚合全部错误。
func stopRBACLifecycle(ctx context.Context, stopDispatcher func(context.Context) error, stopWatcher func(context.Context) error, closer userRoleResolverLifecycle) error {
	return errors.Join(
		stopDispatcher(ctx),
		stopWatcher(ctx),
		closer.Close(),
	)
}
