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

// registerRBACLifecycle 先启动用户角色缓存，再 fail-closed 初始化策略，最后启动跨副本 watcher。
func registerRBACLifecycle(params RegisterRBACLifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := params.Runtime.UserRoles.Start(ctx); err != nil {
				return err
			}
			params.Runtime.Initializer.InitializeFailClosed(ctx)
			params.Runtime.Watcher.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return stopRBACLifecycle(ctx, params.Runtime.Watcher.Stop, params.Runtime.UserRoles)
		},
	})
}

// 生命周期辅助函数

// stopRBACLifecycle 聚合 watcher 停止和本地用户角色缓存关闭错误，避免静默丢失清理失败。
func stopRBACLifecycle(ctx context.Context, stopWatcher func(context.Context) error, closer userRoleResolverLifecycle) error {
	return errors.Join(
		stopWatcher(ctx),
		closer.Close(),
	)
}
