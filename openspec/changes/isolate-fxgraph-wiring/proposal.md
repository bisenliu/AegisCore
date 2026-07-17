## Why

当前 `fxgraph` 命令通过完整生产 `AppModule` 和 `fx.Populate(&graph)` 提取 `fx.DotGraph`，而 `fx.New` 会执行全部 `fx.Invoke`，导致图生成过程中可能构造真实 runtime 资源、注册路由和修改进程级状态。依赖图诊断应是安全、可重复、无外部副作用的开发工具操作，避免 PostgreSQL、Redis、tracing、workerpool、Gin mode 和 timezone 等运行时激活行为被 graph 命令间接触发。

## What Changes

- 将 user-service 的 Fx 装配拆分为无运行时激活的 wiring module 与包含 `fx.Invoke`、lifecycle 和进程级初始化的 runtime module。
- 让 `AppModule` 继续组合 wiring 与 runtime，保持正式 `serve` 行为不变。
- 让 `fxgraph` 命令只基于 wiring graph 或专用无副作用 graph root 生成 DOT，避免复用完整生产 App 的 runtime Invoke。
- 为 graph 生成增加防回归测试，覆盖不连接外部依赖、不创建后台执行资源、不改变 timezone 或 Gin mode、不注册真实 route/runtime metrics、不创建 tracing exporter 后台资源。
- 不改变 HTTP API、数据库 schema、OpenAPI 契约、部署资产或正式运行时启动语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 收紧 `fxgraph` CLI 的无副作用生成契约，要求 graph 命令不执行生产 runtime Invoke、不激活真实 runtime 资源。
- `shared-platform-primitives`: 收紧 `common/runtime/fxgraph` helper 的职责边界，要求 helper 只处理已提供的无副作用 Fx option，并不得通过完整服务 runtime graph 间接触发资源构造。

## Impact

- 影响代码：`common/runtime/fxgraph/fxgraph.go`、`user-service/cmd/fxgraph.go`、`user-service/internal/bootstrap/app.go`，以及相关 providers、router 或 feature module 的 Fx module 组合入口。
- 影响测试：新增或调整 user-service graph 命令测试、bootstrap module 组合测试和 common fxgraph helper 测试，验证 graph 生成不触发外部依赖和进程级副作用。
- 不影响 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration、RBAC 权限模型、容器镜像或部署清单。
- 实现需要谨慎保持正式 App 的依赖图完整性，避免为 graph 命令引入测试专用生产 API 或全局可变 hook。
