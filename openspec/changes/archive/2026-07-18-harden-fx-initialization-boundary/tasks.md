## 1. Constructor 错误语义

- [x] 1.1 将 `user-service/internal/features/permission/application/command.NewPermissionCommandService` 的 nil policy change notifier panic 改为返回明确 error，并更新调用方和测试。
- [x] 1.2 将 `user-service/internal/features/role/application/command.NewRoleCommandService` 的 nil policy change notifier panic 改为返回明确 error，并更新调用方和测试。
- [x] 1.3 将 `user-service/internal/features/auth/application/sessions.NewLifecycle` 的 nil token version local invalidator panic 改为返回明确 error，并更新调用方和测试。
- [x] 1.4 增加或更新 auth、permission、role application constructor 测试，断言缺失依赖返回 error 且不 panic。

## 2. Tracing 与 Redis 初始化

- [x] 2.1 调整 `common/runtime/observability/tracing.NewFxProvider`，使 Fx constructor 阶段返回的 provider 拥有非 nil tracer provider，并继续注册 lifecycle stop shutdown。
- [x] 2.2 更新 tracing provider 测试，覆盖 constructor 阶段 `TracerProvider()` 非 nil、配置错误通过 constructor error 暴露、stop 阶段执行 shutdown。
- [x] 2.3 更新 `user-service/internal/providers.NewCacheRedis` 的 tracing provider 依赖检查和错误包装，确保使用服务 tracing provider 且不回退到全局 provider。
- [x] 2.4 更新 Redis provider/datastore 测试，覆盖 Redis instrumentation error 返回、client 关闭、Fx graph 中 cache Redis 可被正常实例化。

## 3. Fx DI 初始化边界

- [x] 3.1 在 `user-service/internal/bootstrap.AppOptions` 加入 `fx.RecoverFromPanics()`，作为 composition root 的 DI 初始化边界保护。
- [x] 3.2 增加 bootstrap 测试，断言 constructor、decorator 或 Invoke 的 panic 被转换为 Fx error，且 `AppOptions` 仍保留配置 supply、logger provider 和 lifecycle timeout 行为。
- [x] 3.3 保持 HTTP handler recovery、workerpool、scheduler、后台 goroutine 和 lifecycle hook 运行期保护边界不变，不引入全局容错兼容路径。

## 4. 验证与收尾

- [x] 4.1 运行相关包测试：`go test ./common/runtime/observability/tracing ./common/runtime/datastore ./user-service/internal/providers ./user-service/internal/bootstrap ./user-service/internal/features/auth/application/sessions ./user-service/internal/features/permission/application/command ./user-service/internal/features/role/application/command`。
- [x] 4.2 运行文档和架构校验：`make user-service-architecture-lint`。
- [x] 4.3 检查 OpenAPI、Ent schema、migration 和部署资产未发生预期外变更；如无 API/schema/deployment 变更，不运行生成命令。
- [x] 4.4 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区，再运行 `make lint`。
- [x] 4.5 在预期变更已暂存后运行 `make verify`，确保最终 drift 检查不被未暂存预期变更阻塞。
- [x] 4.6 验证通过后更新本 `tasks.md` 对应 checkbox 为 `- [x]`，并保留失败或跳过命令的原因记录。
