## 1. Lifecycle Contract

- [x] 1.1 在 `user-service/internal/features/permission/fx_authorization.go` 定义或复用未导出的显式 lifecycle 接口，要求同时提供 `Start(context.Context) error` 和 `Close() error`。
- [x] 1.2 调整 `UserRoleResolverResult`，保留 resolver 与 stats 输出，并显式输出 user-role resolver/cache lifecycle 视图。
- [x] 1.3 确认 resolver/cache provider 只为同一底层实例暴露多个接口视图，不重复构造有状态 cache 或 resolver。

## 2. RBAC Lifecycle Hook

- [x] 2.1 在 `user-service/internal/features/permission/fx_lifecycle.go` 调整 `RegisterRBACLifecycleParams`，将 `UserRoles` 从关闭接口改为显式 lifecycle 接口。
- [x] 2.2 修改 `registerRBACLifecycle` 的 `OnStart`，直接调用 `params.UserRoles.Start(ctx)`，失败时返回错误并跳过 policy 初始化和 watcher 启动。
- [x] 2.3 保持 `OnStop` 继续通过 `stopRBACLifecycle(ctx, params.Watcher.Stop, params.UserRoles)` 聚合 watcher stop 与 resolver/cache close 错误。
- [x] 2.4 移除 `registerRBACLifecycle` 中对启动能力的 type assertion 或兼容探测逻辑。

## 3. Tests

- [x] 3.1 更新 `user-service/internal/features/permission/fx_test.go` 中的 fake 类型和 Fx 装配断言，使测试提供显式 user-role resolver lifecycle。
- [x] 3.2 增加或更新启动失败测试，验证 `UserRoles.Start(ctx)` 返回错误时 engine 不初始化、watcher 不启动。
- [x] 3.3 增加或更新停止测试，验证 watcher stop 失败时仍会 close user-role resolver/cache，且多错误语义不退化。

## 4. Verification

- [x] 4.1 运行相关 permission feature 测试，例如 `go test ./user-service/internal/features/permission/...` 或仓库约定的等价命令。
- [x] 4.2 运行 `make user-service-architecture-lint`，验证 OpenSpec 和架构规则仍通过。
- [x] 4.3 将本次预期代码、OpenSpec artifact 和相关文档变更加到暂存区。
- [x] 4.4 运行 `make lint` 并修复所有失败项。
- [x] 4.5 运行 `make verify` 并修复所有失败项，确认最终没有未暂存的预期 diff 阻塞验证。
