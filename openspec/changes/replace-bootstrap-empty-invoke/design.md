## Context

`user-service/internal/bootstrap/app.go` 的 `RuntimeModule` 负责注册正式运行时需要主动执行的初始化、路由和 lifecycle。当前模块通过 `fx.Invoke(func(*http.Server) {}, func(*PprofServer) {})` 强制 Fx 解析两个 server，以便它们的构造函数注册启动和停止 hook；这种写法虽然有效，但空匿名函数让意图分散在注释和参数类型中，不利于维护。

本次 change 是 user-service bootstrap 内部清理，不改变 feature 行为、HTTP API、数据库、OpenAPI、部署清单、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 用具名函数表达 runtime server 必须被解析的意图。
- 保持 `*http.Server` 与 `*PprofServer` 的 Fx 依赖解析和 lifecycle hook 注册行为不变。
- 更新测试，覆盖 App graph 校验和源码级意图约束，避免回退为空匿名 invoke。

**Non-Goals:**

- 不调整 `NewHTTPServer`、`NewPprofServer`、server lifecycle hook 或路由挂载行为。
- 不新增通用 helper、接口、adapter 或跨模块抽象。
- 不修改配置、HTTP 契约、OpenAPI 生成物、数据库 migration、部署清单或观测面板。

## Decisions

- 决策：在 `bootstrap` 包内新增未导出的 `registerRuntimeServers(_ *http.Server, _ *PprofServer)`，并在 `RuntimeModule` 中使用 `fx.Invoke(registerRuntimeServers)`。
  备选方案：保留两个空匿名函数，仅调整注释。该方案代码变更更少，但核心问题仍存在：强制解析的目的没有体现在符号名称中。

- 决策：不把该函数移入 `providers` 或 `common`。
  备选方案：将 runtime server 注册抽象为 provider 层 helper。该方案会把 composition root 的启动意图泄漏到 provider 边界，且为一次性装配表达引入不必要抽象。

- 决策：测试保持在 `user-service/internal/bootstrap`，以 `fx.ValidateApp` 和源码约束验证装配行为。
  备选方案：新增 runtime 启停集成测试。该方案成本更高，且本次 change 不改变 server 启停语义，现有 graph 校验足以覆盖依赖解析。

## Risks / Trade-offs

- [Risk] 具名函数签名若遗漏 `*http.Server` 或 `*PprofServer`，对应 lifecycle hook 不会被注册。→ Mitigation：保留 `fx.ValidateApp(AppOptions(cfg, AppModule)...)`，并补充测试检查 `RuntimeModule` 使用 `registerRuntimeServers` 且不再包含空匿名 server invoke。
- [Risk] 源码字符串测试可能对格式较敏感。→ Mitigation：只断言稳定符号和被替换的关键模式，不约束完整格式。
- [Risk] 该 change 的行为影响很小，规格增量可能被误解为新增 runtime 行为。→ Mitigation：spec delta 只约束正式 App 的既有 lifecycle 注册链路必须清晰表达和可验证，不改变 server 启停语义。

## Migration Plan

- 实施时直接修改 bootstrap 代码和测试。
- 部署不需要数据迁移、配置迁移或分阶段发布。
- 回滚方式是恢复原 `fx.Invoke` 写法；由于外部行为不变，回滚不涉及数据或兼容性处理。
- 验证方式：运行 `go test ./user-service/internal/bootstrap`；最终合并前根据仓库流程运行 `make lint` 与 `make verify`。

## Open Questions

- 无。
