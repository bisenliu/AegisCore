## Context

Compose 是本地开发和诊断入口，应只默认暴露当前实际需要的服务端口。当前 `deployments/compose/docker-compose.yml` 同时默认开启 pprof 和 gRPC，并暴露宿主端口 `6060`、`19090`，但主架构说明要求 pprof 默认关闭且通过 loopback 或受控端口转发访问，仓库结构也声明当前没有真实入站 gRPC API。

本次 change 横跨 `deployments/compose/`、`common/go.mod`、`common/runtime/datastore/` 和 OpenSpec artifacts。它不改变 HTTP API、数据库 schema、OpenAPI 生成物、RBAC 授权、Kubernetes/Helm 资产或生产镜像结构。

## Goals / Non-Goals

**Goals:**

- 收敛 Compose 默认监听面，只默认暴露 HTTP、数据库、Redis、Jaeger、Prometheus 和 Grafana 等实际本地入口。
- 保持 pprof 默认关闭，并保留 README 中通过显式环境变量临时诊断的说明。
- 在没有真实入站 gRPC API 前，Compose 默认关闭 gRPC 且不发布宿主 gRPC 端口。
- 清理 `common` 的 direct require 声明。
- 用测试固定 Redis command filter 的 omit 语义。

**Non-Goals:**

- 不新增 pprof 或 gRPC 的兼容 profile、兼容端口或双路径配置。
- 不实现入站 gRPC server、feature `transport/grpc` 或 protobuf 契约。
- 不改变 `OpenPostgres` 的 otelsql instrumentation 生产行为。
- 不改变 Ent tracing、Ent metrics 或 Ent SQL log 的生产代码。
- 不修改 Kubernetes、Helm、Prometheus alert 或 Grafana dashboard。

## Decisions

- 决策：Compose 默认删除 pprof 环境变量和端口暴露。
  理由：runtime 默认已经是 `enabled=false` 和 `127.0.0.1:6060`，Compose 不应把诊断 listener 改为 `0.0.0.0` 并发布到宿主。
  备选方案：保留默认启用并更新 README。该方案扩大默认本地监听面，和仓库安全约定不一致。

- 决策：Compose 默认设置 `AEGISCORE_SERVER_GRPC_ENABLED=false` 并删除 `19090:9090`。
  理由：当前没有真实入站 gRPC API，也没有 gRPC server 生命周期 provider；默认暴露端口会误导本地使用者并增加不必要监听面。
  备选方案：保留 gRPC 配置但不暴露端口。该方案仍让配置契约暗示存在本地 gRPC 调试入口。

- 决策：保留本地 Compose 的双层 DB tracing，并在 README 明确说明。
  理由：Compose 是本地诊断编排，Ent span 提供实体级视角，otelsql span 提供 SQL/driver 视角；两层共同保留有利于完整链路排障。
  备选方案：关闭 Ent tracing 或移除 otelsql。该方案会降低诊断粒度，并超出本次收敛本地监听面的主要目标。

- 决策：只新增测试固定 Redis filter 语义，不抽象额外接口。
  理由：生产实现已经简单且语义正确，测试应直接覆盖现有函数，避免为了测试引入冗余生产代码。

## Risks / Trade-offs

- [风险] 依赖 Compose 默认 pprof 端口的本地脚本会失效。→ 缓解：README 保留显式环境变量和受控访问方式。
- [风险] 依赖 `localhost:19090` 的本地 gRPC 调试会失效。→ 缓解：当前没有真实入站 gRPC API；未来引入时应通过单独 change 恢复配置和端口。
- [风险] Redis filter 测试绑定第三方库当前 `DefaultCommandFilter` 行为。→ 缓解：这正是本次测试目的，第三方语义变化时测试应提醒维护者重新评估敏感命令过滤。

## Migration Plan

1. 修改 Compose 环境变量和端口暴露，恢复 pprof/gRPC 默认关闭。
2. 更新 Compose README，删除过时端口暗示并补充本地 tracing 分层说明。
3. 移动 `github.com/XSAM/otelsql` 到 `common/go.mod` direct require。
4. 新增 Redis command filter 单元测试。
5. 运行 `go test ./runtime/datastore`、`docker compose -f deployments/compose/docker-compose.yml config`、`make user-service-architecture-lint`，按影响范围运行 `make lint` 与 `make verify`。

回滚方式：回滚本 change 的 Compose、README、go.mod、测试和 OpenSpec artifacts；不需要数据迁移或运行时兼容逻辑。

## Open Questions

无。
