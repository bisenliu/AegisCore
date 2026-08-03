## Why

`user-service/internal/providers/` 当前集中承载 Gin、routes、PostgreSQL、Redis、Ent、JWT、健康检查、metrics、tracing 和限流等多类服务级接线职责，目录内已有 30+ 个 Go 文件。继续保持单目录和单 package 会增加新成员理解成本，并在多人并行维护 datastore、security、observability 和 HTTP transport 接线时放大冲突与误改风险。

## What Changes

- 将 `user-service/internal/providers/` 从单一大 package 拆分为按关注点组织的子包。
- 新增 `providers/datastore/`，承载 PostgreSQL、Redis、Ent client、Ent plugins、Ent SQL log、Ent metrics 和 Ent tracing 接线。
- 新增 `providers/observability/`，承载 health checks、runtime dependency metrics、Prometheus/OpenTelemetry provider 接线入口。
- 新增 `providers/security/`，承载 JWT service、认证 token policy 和 password service 接线入口。
- 新增 `providers/transport/`，承载 Gin engine、HTTP routes、rate limiters 和传输层 middleware 接线。
- 保留 `providers` 根包作为 composition 汇总入口，只暴露 `WiringModule`、`RuntimeModule` 和 `Module`。
- 迁移相关测试文件到对应子包，删除旧单包测试组织方式。
- 更新架构文档和能力地图中的 provider 路径说明。
- **BREAKING**：不保留旧 `providers` 包内具体 provider 构造器的兼容 wrapper、alias 或兼容分支；内部引用必须直接改为新子包路径。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `opsx-foundation`：补充架构文档、能力地图和 change artifacts 对服务级 provider 物理边界的治理要求，确保目录结构与正式架构来源保持一致。

## Impact

- 影响代码：`user-service/internal/providers/`、`user-service/internal/bootstrap/app.go` 以及迁移后的 provider 测试。
- 影响文档：`docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 中关于 `user-service/internal/providers/` 的说明。
- 不影响 HTTP API、OpenAPI 输出、数据库 schema、Atlas migration、RBAC 权限基线、部署 YAML/Helm/Compose、Prometheus alert 或 Grafana dashboard。
- 需要重点验证 Fx graph 组装、user-service provider package 测试和架构 lint，确保拆包后依赖方向仍满足现有边界约束。
