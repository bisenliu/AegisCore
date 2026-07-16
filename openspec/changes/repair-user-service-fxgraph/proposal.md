## Why

`user-service` 的 `fxgraph` 诊断命令当前因缺少 `*serviceconfig.Config` 等正式 App 基础输入而确定性失败，导致依赖图资产和运行时 App 图无法被可靠验证。该问题会让诊断图与正式运行图继续漂移，降低 Fx provider 变更、配置统一和交付门禁的可信度。

## What Changes

- 修复 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot` 缺少服务配置而失败的问题。
- 让 `fxgraph` 与正式 App 共用 `unify-user-service-app-configuration` 提供的基础 input/options builder，并在命令层补齐 service config、派生 runtime config、logger 与资源替身。
- 保持 `common/runtime/fxgraph` 只负责通用 DOT rendering，不引入 user-service 配置、provider 或 feature 依赖。
- 将 `fxgraph` 测试从仅断言 option 数量升级为实际调用 `RenderDOT` 的 smoke test，断言关键 AppModule、feature 节点与依赖边存在，并在关键依赖缺失时失败。
- 更新或重新生成版本控制中的 Fx dependency graph 资产，并复用或补充 drift 检查目标。
- 不连接真实 PostgreSQL、Redis、OTLP，不启动 HTTP server，不改变正式运行依赖图的业务行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 明确通用 `common/runtime/fxgraph` 只承担业务无关的 DOT 渲染职责，服务私有输入与 provider 组装不得下沉到 common。
- `delivery-operations`: 明确 user-service 的 `fxgraph` 诊断命令、测试和受版本控制图资产必须能反映正式 App provider 图并通过 drift 校验。

## Impact

- 受影响代码包括 `user-service/cmd` 中的 `fxgraph` 命令、相关测试和可能存在的 Fx graph 生成或检查脚本/Makefile 目标。
- 受影响共享边界包括 `common/runtime/fxgraph` 的职责约束，但不要求该包导入 user-service 私有代码。
- 受影响交付资产包括版本控制中的 Fx dependency graph DOT 资产以及对应生成或 drift 检查流程。
- 不影响 HTTP API、OpenAPI、数据库 schema、Ent migration、RBAC 业务语义、认证会话语义或生产服务启动行为。
