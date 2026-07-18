package permission

import (
	"context"
	"errors"

	"go.uber.org/fx"

	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
)

// permissionLifecycleOptions 只注册运行时 hook，便于测试和 graph 生成单独使用 WiringModule。
var permissionLifecycleOptions = fx.Options(
	fx.Invoke(
		registerRBACLifecycle,
	),
)

// RegisterRBACLifecycleParams 汇集 RBAC 启停依赖，启动顺序由 registerRBACLifecycle 统一控制。
type RegisterRBACLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Engine    permissionPolicyInitializer
	Watcher   permissionApplicationWatcher
	Closer    permissioncasbin.UserRoleCacheCloser
	Starter   userRoleResolverStarter `optional:"true"`
}

// registerRBACLifecycle 先启动用户角色缓存，再 fail-closed 初始化策略，最后启动跨副本 watcher。
func registerRBACLifecycle(params RegisterRBACLifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if params.Starter != nil {
				if err := params.Starter.Start(ctx); err != nil {
					return err
				}
			} else if params.Closer == nil {
				return errors.New("rbac user role cache lifecycle dependency is required")
			}
			params.Engine.InitializeFailClosed(ctx)
			params.Watcher.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return stopRBACLifecycle(ctx, params.Watcher.Stop, params.Closer)
		},
	})
}

// stopRBACLifecycle 聚合 watcher 停止和本地用户角色缓存关闭错误，避免静默丢失清理失败。
func stopRBACLifecycle(ctx context.Context, stopWatcher func(context.Context) error, closer permissioncasbin.UserRoleCacheCloser) error {
	return errors.Join(
		stopWatcher(ctx),
		closer.Close(),
	)
}
