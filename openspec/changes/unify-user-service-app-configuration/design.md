## Context

`user-service/cmd/serve.go` 当前先调用 `serviceconfig.NewConfig(ConfigPath(configPath))`，用解析结果中的 `runtime.lifecycle.start_timeout` 和 `stop_timeout` 构造手动 `App.Start`/`App.Stop` context；随后 app factory 只接收路径并调用 `bootstrap.NewApp(configPath)`。`user-service/internal/bootstrap/app.go` 又向 Fx supply `ConfigPath`，并以 `serviceconfig.NewConfig`、`serviceconfig.NewRuntimeConfig` provider 重新读取同一文件。这样一次启动存在两次独立 I/O，文件若在两次读取之间变化，CLI budget、service config、共享 runtime config 和 logger/resource provider 可能来自不同快照。

Fx 的 `fx.StartTimeout` 与 `fx.StopTimeout` 是 App 顶层 lifecycle 默认配置，但显式调用 `App.Start(ctx)` 和 `App.Stop(ctx)` 时，实际 deadline 由调用方传入的 context 决定。`fx.New` 会同步构建并校验依赖图、执行 invoke 及其依赖构造；它不受 `fx.StartTimeout` 限制。当前部分资源已经把可取消工作放入 lifecycle hook，但本 change 不迁移全部构造期工作。

本 change 影响 `user-service/cmd` 与 `user-service/internal/bootstrap` 的装配入口、`internal/providers` 和 bootstrap/cmd 的相关测试，以及文档和 OpenSpec。`common/runtime/config` 的字段、默认值、校验与 loader API 保持不变；基础 App options 属于 user-service composition root，不进入 `common`、`internal/shared`、`internal/integration` 或 feature 包。`deployments`、HTTP/OpenAPI、Ent/Atlas、观测资产和安全边界不变。进行中的 `align-user-service-termination-budget` 仍独立负责部署 grace 与 Stop 总预算关系，本 change 只保证 App 与 CLI 消费同一预算来源。

## Goals / Non-Goals

**Goals:**

- 让一次 `serve` 启动只解析一次 service config，并让 CLI lifecycle context、service providers 和共享 runtime providers 消费同一配置快照。
- 提供无文件 I/O、可组合的 user-service 基础 Fx options 构建入口，使正式 `NewApp`、装配测试和后续诊断命令共享同一 composition root。
- 将配置化 Start/Stop timeout 同时设置到 Fx App 顶层和 CLI 显式 lifecycle context，并准确记录二者的作用边界。
- 通过指针身份、派生 runtime config、App timeout 和依赖图装配测试阻止第二套配置 provider 链回归。

**Non-Goals:**

- 不声称或尝试用 `fx.StartTimeout` 限制配置加载、`fx.New`、provider constructor 或 invoke 的同步构造阶段。
- 不在本 change 把所有构造期资源 I/O 迁移到 `OnStart`，不重排 feature、provider 或 lifecycle hook。
- 不修复或迁移 `fxgraph` 命令；只让基础 options 入口可供后续 change 复用。
- 不改变配置字段、默认值、环境变量、业务 API、OpenAPI、数据库 schema、migration、部署清单、指标、日志字段或认证/RBAC 行为。

## Decisions

### Decision: CLI 拥有 serve 配置 I/O 并把已解析对象传给 App factory

`runServe` 保留对 `serviceconfig.NewConfig` 的唯一调用。`lifecycleAppFactory` 改为接收 `*serviceconfig.Config`，正式 factory 将同一对象传给 `bootstrap.NewApp`；CLI 从该对象读取 lifecycle budget 并创建 Start/Stop context。配置解析失败继续在创建 Fx App 之前返回，因此不会产生部分依赖图或需要关闭的资源。

选择传递已解析对象而不是路径，是为了让类型签名直接表达单一配置快照所有权，并让测试能断言 factory 收到同一指针。备选方案是在 Fx 中保留 loader、同时让 CLI 只读取 lifecycle 子集，仍会产生两次 I/O 和快照漂移；引入进程级配置缓存会增加并发、失效和测试隔离问题，也不采用。

### Decision: bootstrap 提供纯基础 Fx options builder

在 `user-service/internal/bootstrap` 提供接收非空 `*serviceconfig.Config` 并返回基础 `[]fx.Option` 的构建入口。该入口直接 `fx.Supply` service config 及通过 `serviceconfig.NewRuntimeConfig(cfg)` 派生的共享 runtime config，接入 `logger.NewLogger`、`AppModule`，并设置来自同一对象的 `fx.StartTimeout` 与 `fx.StopTimeout`。`NewApp(cfg)` 只负责把这些 options 交给 `fx.New`，不接收路径、不加载文件，也不保留 `serviceconfig.NewConfig` 或 `ConfigPath` provider。

该 builder 是 user-service composition root 的正式 API，测试可在其基础上增加 `fx.NopLogger`、`fx.Populate`、替代 module 或验证 option，而不复制 service/runtime config 接线。`AppModule` 继续表达 feature、provider、HTTP/pprof 和 invoke 图；`providers.Module` 不拥有配置加载。备选方案只抽取 config options、让正式 App 与测试各自拼 logger/module/timeout，会形成第二套装配清单；把该 builder 放入 `common` 则会泄漏 user-service module 与私有配置语义，均不采用。

### Decision: App 顶层 timeout 与 CLI 显式 context 使用同一数值但职责不同

基础 options 设置 `fx.StartTimeout(cfg.Runtime.Lifecycle.StartTimeout)` 和 `fx.StopTimeout(cfg.Runtime.Lifecycle.StopTimeout)`，使 `App.StartTimeout()`、`App.StopTimeout()` 及未来采用 Fx 默认运行入口的调用方与配置一致。现有 CLI 继续为手动 `App.Start(startCtx)` 和 `App.Stop(stopCtx)` 创建相同预算的 context；这些显式 context 才是当前 serve 路径实际传给 lifecycle hook 的 deadline，并继续保留停止时的上游 context value。

文档和注释必须明确：配置解析发生在 `fx.New` 之前，`fx.New` 的同步依赖构造也不由 `fx.StartTimeout` 截止。备选方案只设置 App options 并删除 CLI context，会改变手动调用的 deadline 所有权；只保留 CLI context 而不设置 App options，则 App 自身暴露的 timeout 与配置不一致，也不利于后续复用，因此都不采用。

### Decision: 以装配契约测试验证单一来源，不增加测试专用生产 hook

`cmd` 测试更新 factory 签名并断言收到的 service config 指针及 lifecycle 数值，覆盖解析错误时不创建 App、成功路径的 Start/Stop context budget 和单次 Stop 语义。`bootstrap` 测试使用基础 options 验证同一 service config 指针和由其派生的 runtime config 可被解析，且 App 的 Start/Stop timeout 与配置一致；`providers` 相关装配测试复用该入口或明确 supply 已解析配置，不再建立 `ConfigPath -> NewConfig` 链。

测试不向正式代码加入可变 loader、调用计数器或 test-only option。单次读取由 `runServe` 的唯一 loader 调用、factory 的配置对象入参和 bootstrap 无 I/O API 共同保证，并通过源码/装配回归测试约束。

## Risks / Trade-offs

- [Risk] `NewApp` 与 factory 签名变化会影响 cmd、测试和 E2E harness 的现有调用点 -> Mitigation：全仓搜索并统一迁移到已解析配置或基础 options，运行指定包测试与 `make verify`。
- [Risk] service config 与派生 runtime config 是两个指针，后续误改 service config 可能造成投影漂移 -> Mitigation：在 composition root 中紧邻、单次派生并 supply，两者在 App 构建后按只读配置使用；测试验证关键 lifecycle 值一致。
- [Risk] 同时配置 Fx timeout 和 CLI context 容易被误解为两层可累加预算 -> Mitigation：文档明确数值同源但当前手动调用只由传入 context 决定，二者不是串行或累加 timeout。
- [Risk] `fx.New` 中仍可能执行耗时或阻塞的 constructor/invoke -> Mitigation：明确排除虚假 timeout 保证；构造期资源迁移必须基于独立盘点和后续 change。
- [Risk] `fxgraph` 暂时仍使用独立装配链 -> Mitigation：不在本 change 改变其行为；基础 options builder 保持可组合，后续诊断 change 可显式迁移并处理真实配置与副作用隔离。

## Migration Plan

1. 先新增基础 App options builder 和装配测试，将 `NewApp` 改为消费已解析配置并设置 Fx timeout。
2. 再迁移 `serve` factory 签名和测试，使 CLI 唯一加载配置并把对象传入 composition root；同步调整 E2E/validation/provider 装配调用点。
3. 更新配置注释及 architecture/development/testing 中相关说明，明确单一配置来源、手动 lifecycle context 与 `fx.New` 边界。
4. 运行 `cd user-service && go test ./cmd ./internal/bootstrap ./internal/providers -count=1`、`openspec validate unify-user-service-app-configuration` 和 `make user-service-architecture-lint`；完成实现与规格后仅暂存本 change 预期文件，再运行 `make lint` 与 `make verify`。

该变更不需要数据、配置或部署迁移，随正常 user-service 构建发布。回滚时可整体恢复 `NewApp(configPath)` 和旧 factory 签名，但会重新引入重复配置读取；不涉及数据库、API 或部署资产回滚。

## Open Questions

无。基础 options builder 归属 bootstrap，正式 serve 配置 I/O 归属 CLI，`fxgraph` 迁移留待后续 change。
