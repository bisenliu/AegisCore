## Context

`common/runtime/workerpool` 是跨服务共享 runtime primitive，当前 `New(lc fx.Lifecycle, log *zap.Logger, opts Options)` 在构造时可直接向 Fx lifecycle 追加 `OnStop` hook。这个设计把普通后台任务池和具体 DI/lifecycle 框架耦合，和 `common` 只承载无业务语义 primitive 的边界不一致，也阻碍后续 auth 显式生命周期和顶层 Runtime 解耦。

当前已知生产调用点集中在 auth Redis session purge pool：`user-service/internal/features/auth/infrastructure/redis/session_purge_pool.go` 通过 `workerpool.New(params.Lifecycle, ...)` 创建命名池，并由 Fx hook 保证在 Redis client 停止前 drain。变更后 workerpool 自身不再知道 Fx，user-service 可在服务私有 composition 中继续使用 Fx，但必须显式登记 `pool.Stop(ctx)`。

本变更影响 Go 代码、OpenSpec 规格和测试；不影响 HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或安全契约。`common/runtime/workerpool` 不得导入 `go.uber.org/fx` 或 `fxtest`。

## Goals / Non-Goals

**Goals:**

- 让 `workerpool.New` 成为普通 Go 构造器，只接收 logger 和 `Options`，返回需要调用方显式关闭的 `*Pool`。
- 从 `common/runtime/workerpool` 生产代码和测试中移除 `go.uber.org/fx`、`fx.Lifecycle`、`fx.Hook` 与 `fxtest` 依赖。
- 保持并发上限、阻塞提交、任务 context 联动、panic recovery、统计、幂等 drain、StopTimeout 和错误语义不变。
- 在 auth session purge pool 当前调用点中迁移关闭责任，由 user-service 私有 Fx composition 显式注册 `pool.Stop(ctx)`，保证关闭顺序仍满足 purge pool 先于 Redis client 停止。
- 用规格 delta 明确 workerpool 的调用方所有权和显式关闭责任。

**Non-Goals:**

- 不修改 session purge 业务语义、Redis key、任务内容、worker 数、StopTimeout 或指标语义。
- 不重写 user-service 顶层启动/关闭，不引入通用 lifecycle 框架、DI 容器或 service locator。
- 不修改 scheduler、localcache 或其他 runtime primitive。
- 不保留旧签名、deprecated wrapper、可选 lifecycle 参数或兼容 adapter。
- 不新增数据库 migration、OpenAPI 输出、部署资产或观测 dashboard。

## Decisions

### Decision: workerpool 构造器只保留普通 Go 输入

`workerpool.New` 改为接收 `log *zap.Logger` 和 `opts Options`，内部直接创建 `Pool`。现有 `newUnmanaged` helper 可以内联或降为包内实现细节，但不再作为“绕过 lifecycle”的测试语义存在。

备选方案：保留 `NewWithLifecycle` 或可选 `Lifecycle` 参数。拒绝原因是本 change 明确是不兼容 API 收紧，保留兼容入口会继续让 `common/runtime/workerpool` 携带 Fx 语义，削弱后续 Runtime 解耦收益。

### Decision: 显式关闭责任归属调用方资源 owner

workerpool 自身只暴露 `Stop(ctx)` 并维持现有 drain 语义。任何创建 `Pool` 的生产 owner 必须在自身生命周期边界注册或调用 `Stop(ctx)`，例如 user-service 在 auth Redis infrastructure provider 中构造池后，通过服务私有 Fx hook 注册关闭。

备选方案：在 `common/runtime/workerpool` 提供通用 lifecycle interface。拒绝原因是会用新的抽象替代 Fx 耦合，增加 runtime primitive 的生命周期框架假设，不符合本次只移除 Fx 依赖的目标。

### Decision: auth 迁移留在服务私有 composition 边界

`NewSessionPurgePool` 仍属于 auth Redis infrastructure 装配边界，可以接收 Fx lifecycle 作为 user-service 私有 provider 输入，在创建 workerpool 后直接 `Append(fx.Hook{OnStop: pool.Stop})`。这个 Fx 依赖不得下沉回 `common/runtime/workerpool`。

备选方案：把关闭注册移动到顶层 bootstrap 或全局 Runtime。拒绝原因是本 change 不重写顶层生命周期；auth purge pool 的资源 owner 和 Redis 关闭顺序需求当前就在 auth infrastructure/provider 测试中表达，局部迁移最小且可验证。

### Decision: 测试从 lifecycle fixture 改为行为覆盖

`common/runtime/workerpool` 测试删除 `fxtest`，通过直接调用 `New`、`Submit`、`Stop` 验证构造失败、关闭后拒绝、panic recovery、StopTimeout、重复 Stop 和完整 drain。auth Redis 测试保留服务私有 Fx 关闭顺序覆盖，断言 purge pool `Stop` hook 仍先于 Redis stop hook。

备选方案：仅删除 lifecycle 测试不补行为覆盖。拒绝原因是 workerpool 的稳定契约集中在 drain、拒绝、panic 和超时语义，API 解耦必须证明这些行为没有回退。

## Risks / Trade-offs

- [Risk] 调用方忘记 `Stop(ctx)` 会导致后台任务池或 goroutine 生命周期泄露。→ 通过规格要求、auth provider 显式 hook、测试和调用点搜索验证每个生产 owner 都登记关闭。
- [Risk] auth purge pool 关闭顺序变化可能让清理任务在 Redis client 关闭后继续运行。→ 保留或调整 `TestSessionStorePurgePoolStopHookPrecedesRedisStopHook`，验证服务私有 hook 顺序仍正确。
- [Risk] 删除旧签名会破坏未发现的生产或测试调用点。→ 使用 `rg 'workerpool\.New|go\.uber\.org/fx|fx\.|fxtest' common/runtime/workerpool user-service/internal/features/auth` 和相关包测试验证。
- [Risk] 直接迁移测试可能只覆盖实现细节而非契约。→ 任务要求覆盖构造失败、拒绝、panic、超时、重复 Stop 和完整 drain，并运行指定 package 测试。

## Migration Plan

1. 更新 `shared-platform-primitives` delta spec，明确 workerpool 不再由 Fx hook 托管，调用方拥有 `Stop(ctx)` 责任。
2. 修改 `common/runtime/workerpool` 构造器签名和注释，删除 Fx import、hook 注册和 `fxtest` 测试依赖。
3. 迁移 auth Redis session purge pool provider：先调用新的 `workerpool.New`，再在 user-service 私有 Fx lifecycle 中注册 `pool.Stop(ctx)`。
4. 更新相关测试，确保 workerpool 行为和 auth purge pool 关闭顺序保持不变。
5. 运行验收命令：`rg -n 'go\.uber\.org/fx|fx\.|fxtest' common/runtime/workerpool`、`cd common && go test ./runtime/workerpool -count=1`、`cd user-service && go test ./internal/features/auth/infrastructure/redis -count=1`、`openspec validate detach-workerpool-from-fx-lifecycle`、`make user-service-architecture-lint`。
6. 完成实现和文档任务并暂存预期变更后，运行 `make lint` 与 `make verify`。

Rollback 策略：如实施期间发现 auth 关闭顺序无法局部保证，回滚代码修改并保留 change artifacts 待顶层 Runtime change 重新设计；不通过恢复旧 workerpool Fx 签名作为长期方案。

## Open Questions

无。
