## Context

`user-service/internal/providers/` 是 user-service 的服务级 composition/provider 接线层。当前该目录同时承载 datastore、security、observability 和 HTTP transport 接线，目录内已有 30+ 个 Go 文件，且生产代码和测试都处于同一个 `providers` package。`docs/ARCHITECTURE.md` 与 `docs/opsx/CAPABILITY_MAP.md` 已将该目录列为关键服务级装配入口，因此本次调整需要同步代码结构、测试组织和正式架构文档。

当前外部生产引用主要集中在 `user-service/internal/bootstrap/app.go`，其通过 `providers.WiringModule` 与 `providers.RuntimeModule` 接入正式 Fx graph。该入口应保持为 composition root 的稳定聚合点，但不应继续把所有具体 provider 构造器留在根包。

## Goals / Non-Goals

**Goals:**

- 将服务级 provider 物理拆分为 `datastore`、`observability`、`security` 和 `transport` 四个关注点子包。
- 保持 `providers` 根包只负责组合子模块并暴露 `WiringModule`、`RuntimeModule` 和 `Module`。
- 迁移对应测试，使测试路径与被测关注点一致。
- 同步更新 `docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和 OPSX 规格约束。
- 保持 HTTP API、数据库 schema、OpenAPI、部署资产、metrics/tracing/health 输出语义和安全策略不变。

**Non-Goals:**

- 不改变 user/auth/permission/role feature 的分层或业务代码归属。
- 不移动 `common/` 中的共享 runtime、security、observability primitive。
- 不引入新的 DI 框架、Fx value group 路由注册模式或运行时兼容分支。
- 不新增 Ent schema、Atlas migration、OpenAPI 注解、部署清单或观测资产。

## Decisions

### Decision: 根包只保留模块汇总

`user-service/internal/providers` 根包 MUST 只保留 Fx module 汇总职责。`providers/fx.go` 引入四个子包并组合它们的 `WiringModule` 与 `RuntimeModule`，`bootstrap` 继续只依赖根包。

备选方案：让 `bootstrap` 直接依赖四个子包。该方案会把 provider 分组细节泄漏到顶层 App composition root，后续调整子包会增加 `bootstrap` 变更频率，因此不采用。

### Decision: 四个子包按实际关注点拆分

`providers/datastore` 承载 PostgreSQL、Redis、Ent client、Ent plugins、Ent SQL log、Ent metrics 和 Ent tracing。`providers/observability` 承载 health checks、runtime dependency metrics，以及 metrics/tracing provider 的 Fx 接线入口。`providers/security` 承载 JWT service、认证 token policy 和 password service 接线入口。`providers/transport` 承载 Gin mode、Gin engine、routes 和 API rate limiters。

备选方案：将 health checks 放入 `transport` 或 `datastore`。health checks 同时读取 datastore 和 RBAC watcher 状态，面向 readiness/startup 观测端点，更适合归入 `observability`，因此不采用其他归属。

### Decision: 不保留兼容 wrapper 或 alias

本变更只影响 `internal` 代码，旧 `providers.NewPrimaryDB`、`providers.NewGinEngine` 等具体符号不保留 wrapper、type alias 或兼容分支。所有内部测试和引用一次性迁移到新子包。

备选方案：在根包保留旧符号转发。该方案会延续根包职责过宽问题，并让新旧路径长期共存，不符合本次整改目标，因此不采用。

### Decision: 测试随生产代码迁移

与 Gin、routes、ratelimit 相关的测试进入 `providers/transport`；与 PostgreSQL、Redis、Ent 相关的测试进入 `providers/datastore`；与 auth/JWT 相关的测试进入 `providers/security`；与 health/metrics 相关的测试进入 `providers/observability`。仅测试根包 module 组合的测试保留在 `providers` 根包。

备选方案：保留所有测试在根包以减少移动量。该方案无法降低查找成本，也会继续让根包测试依赖多个关注点细节，因此不采用。

## Risks / Trade-offs

- Go package 拆分后原包内未导出 helper 不可跨包访问 → 通过把 helper 放入其唯一消费子包，避免新增共享 helper 包。
- Fx module 顺序或 Invoke 位置迁移可能改变启动链路 → 保持原有 `WiringModule` 和 `RuntimeModule` 语义，并用 provider 相关测试与 `make user-service-architecture-lint` 验证。
- 测试迁移可能暴露包内私有符号依赖 → 优先让测试与被测生产代码在同一子包内，避免为测试新增生产 API。
- 文档与能力地图可能和新目录结构 drift → 同一 change 中更新 `docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和规格 delta。
- 拆包本身会产生较多移动 diff → 不混入业务逻辑改写，降低审查范围。

## Migration Plan

1. 创建四个 provider 子包和各自 `fx.go`。
2. 按关注点移动生产文件，并更新 package 名、imports 和 Fx module 组合。
3. 按关注点移动测试文件，并删除旧根包对具体 provider 的直接测试依赖。
4. 更新 `docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和相关 OpenSpec delta。
5. 执行 `go test ./user-service/internal/providers/...`、`go test ./user-service/internal/bootstrap` 和 `make user-service-architecture-lint`。

回滚方式：该 change 不涉及持久化数据、API 或部署资源。若实施后验证失败，直接回退本 change 的代码移动与文档更新即可，无需数据迁移回滚或运行时兼容分支。

## Open Questions

无。
