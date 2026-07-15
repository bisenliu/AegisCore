## Why

user-service 的 `serve` 命令与 `bootstrap.NewApp` 当前会针对同一路径分别加载 service config：CLI 用第一次解析结果建立 Start/Stop context，Fx composition root 又通过 provider 链执行第二次 I/O。两次读取可能得到不同内容或不同错误，使 lifecycle budget 与正式 App 实际消费的配置失去单一来源，也阻碍测试和后续诊断入口复用真实装配。

## What Changes

- 让 `serve` 命令只加载一次 `*serviceconfig.Config`，并将该对象及其派生的共享 runtime config 通过 `fx.Supply` 交给 user-service composition root，移除正式 App 内第二套配置文件 provider 链。
- 提取边界清晰、无配置 I/O 的基础 Fx options builder，统一承载已解析配置、logger 和 App module 装配，供 `NewApp`、装配测试及后续诊断命令复用。
- 在正式 App 顶层根据同一份已解析配置设置 `fx.StartTimeout` 与 `fx.StopTimeout`；CLI 仍使用配置化 Start/Stop context 手动调用 `App.Start` 和 `App.Stop`，作为实际生命周期调用边界。
- 增加单次配置加载、options 复用、派生 runtime config 一致性和 Fx timeout 接线的回归测试，并修正文档与注释，明确 `fx.StartTimeout` 不限制 `fx.New` 的同步构造阶段。
- 保持现有配置字段、默认值、环境变量、业务 API、资源构造与运行时行为不变；不在本 change 迁移全部构造期资源，也不修复 `fxgraph` 命令。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 明确 service config 在正式启动路径中只解析一次，服务 composition root 必须消费已解析配置及派生 runtime config，并提供无重复 I/O、可供正式 App 与测试复用的基础 Fx options 构建入口。
- `runtime-observability`: 明确 user-service 顶层 Fx App 的 Start/Stop timeout 与 CLI lifecycle context 必须来自同一份配置，并区分 Fx lifecycle timeout 与 `fx.New` 同步构造阶段的边界。

## Impact

- 受影响代码：`user-service/cmd/serve.go`、`user-service/internal/bootstrap/app.go`、`user-service/internal/providers/` 的装配测试及相关 `cmd`、`bootstrap` 测试。
- 受影响文档与规格：composition root 和 Fx lifecycle timeout 说明，以及 `shared-platform-primitives`、`runtime-observability` delta specs。
- 不改变配置字段、默认值、环境变量契约、HTTP/OpenAPI、认证/RBAC、数据库 schema、Atlas migration、外部依赖版本、Kubernetes/Helm 终止预算或业务行为。
- 不修改 Ent 或 OpenAPI 生成物，不新增配置 I/O helper 到 `common/`、`internal/shared` 或 feature 包。
