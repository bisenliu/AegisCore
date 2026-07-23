## Why

当前 Compose 本地编排默认启用了 pprof 和 gRPC，并把 `6060`、`19090` 暴露到宿主机，但仓库架构约定中 pprof 是默认关闭的独立诊断监听，当前也没有真实入站 gRPC API。同时 `common` 中 PostgreSQL tracing 直接依赖 `github.com/XSAM/otelsql`，但 `go.mod` 仍把它标记为 indirect；Redis tracing command filter 依赖第三方 `WithCommandFilter` 的布尔语义，缺少针对敏感命令和健康命令的单元测试。需要收敛本地默认监听面并补齐依赖和测试卫生。

## What Changes

- Compose 默认关闭 pprof，不再声明 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED=true`、`AEGISCORE_OBSERVABILITY_PPROF_ADDR=0.0.0.0:6060`，也不再暴露 `6060:6060`。
- Compose 默认关闭 gRPC，不再声明 `AEGISCORE_SERVER_GRPC_ENABLED=true`，也不再暴露 `19090:9090`。
- Compose README 保持 pprof 临时诊断说明，并明确本地 tracing 默认包含 Ent 实体级 span 与 PostgreSQL SQL/driver 级 span。
- `github.com/XSAM/otelsql` 作为 `common` direct require 声明。
- 增加 Redis command filter 单元测试，固定 `auth`、`hello ... auth`、`ping` 和普通命令的过滤语义。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`: 调整 Compose 本地默认服务监听面和端口暴露。
- `runtime-observability`: 明确 pprof 默认关闭、Redis command filter 测试语义和本地 tracing 分层说明。
- `shared-platform-primitives`: 清理 `common` 直接依赖声明并补充 Redis runtime primitive 测试。

## Impact

- 部署影响：Compose 本地默认不再发布 pprof 和 gRPC 端口；需要 pprof 时必须显式临时开启并通过受控通道访问。
- 观测影响：本地 Compose 仍启用 metrics、tracing、Ent SQL log、Ent tracing 和 Ent metrics；README 说明会明确 Ent span 与 otelsql span 的分层定位。
- 代码影响：仅新增 `common/runtime/datastore` 单元测试，不改变 Redis/PostgreSQL/Ent 生产逻辑。
- 依赖影响：`common/go.mod` 中 `otelsql` 从 indirect require 移到 direct require。
- API、数据库和 OpenAPI 影响：不改变 HTTP API、数据库 schema、Atlas migration 或 OpenAPI 生成物。
