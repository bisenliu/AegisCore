## Context

当前测试覆盖包含 Repository 单元/切片测试、Controller 测试、认证中间件测试、trace-id/recovery/request logger 测试，以及 `miniredis` 驱动的 Redis repository 测试。这些测试能覆盖单层行为，但跨层链路仍较分散：Repository 边界条件未系统化，完整 Gin engine + route registration + auth + trace-id + request logging + recovery + controller/service/repository 的贯通路径不足，Redis token version cache 与 HTTP 认证中间件之间也缺少端到端验证。

本变更涉及测试代码和测试依赖组织，主要影响 `user-services/internal/repository/postgres/`、`user-services/internal/repository/redis/`、`user-services/internal/bootstrap/`、`user-services/internal/router/`、`user-services/internal/controller/`、`user-services/internal/service/` 和 `common/http/middleware/`。实现必须保持 controller/service/repository 分层，不把 HTTP 解析、业务编排或数据访问职责混入测试 helper 的生产代码中。

## Goals / Non-Goals

**Goals:**

- 补齐用户服务 Repository 集成测试，覆盖 CRUD 已有能力、列表边界、唯一性冲突、not found、token version 读取与递增等跨 Ent repository 行为。
- 补齐 HTTP 端到端测试，通过真实 Gin engine、真实路由注册和共享中间件链验证统一响应信封、认证边界、trace-id 传播、请求日志字段和 recovery 500 行为。
- 补齐 Redis token version cache 集成测试，使用 `miniredis` 验证缓存 miss/backfill/TTL/失效和旧 token 经 HTTP 认证中间件被拒绝的贯通路径。
- 保持测试快速、确定性和可在本地 `go test ./...` 中运行；仅在确有 PostgreSQL 特有语义需要验证时引入真实 PostgreSQL 容器测试。

**Non-Goals:**

- 不修改 HTTP API path、method、响应信封、错误码或认证失败公开文案。
- 不修改 Ent schema、Atlas migration、数据库字段或运行时配置格式。
- 不新增授权或细粒度权限功能；403 仅用于验证现有或可注入授权失败响应路径的测试契约，不承诺新增业务授权能力。
- 不把测试 helper 升级为生产 runtime 依赖，不为测试绕开已有 bootstrap、router、中间件或 repository 边界。

## Decisions

- Repository 测试默认使用 Ent SQLite 测试库。
  - 理由：现有 repository 测试已经具备 SQLite 基础，运行快且不要求 Docker，适合覆盖通用 CRUD、唯一性、not found、列表分页和 token version 行为。
  - 替代方案：所有 Repository 测试改用 `testcontainers-go` PostgreSQL。该方案更接近生产，但显著增加本地和 CI 成本，且当前需求大多不是 PostgreSQL 方言特有行为。

- PostgreSQL 容器测试采用按需策略。
  - 理由：只有当测试目标依赖 PostgreSQL 唯一索引、事务隔离、SQL 方言或连接池行为时，才需要 `testcontainers-go`。否则优先保持快速测试路径。
  - 替代方案：完全不使用 PostgreSQL 容器。该方案成本最低，但无法为 PostgreSQL 特有回归提供防护。

- HTTP 集成测试通过真实 Gin engine 和路由注册入口执行。
  - 理由：完整链路问题通常出现在中间件顺序、路由分组、响应信封和 controller/service/repository 协作处，仅测试单个 handler 无法覆盖这些风险。
  - 替代方案：继续使用 controller 或 middleware 单元测试拼接覆盖。该方案已有覆盖，但无法证明真实服务路由链路等价。

- Redis token version 集成测试使用 `miniredis`，并尽量组合真实 token service、真实认证中间件和 repository 抽象。
  - 理由：`miniredis` 已在仓库中使用，能稳定验证 Redis key、TTL 和缓存语义；与 HTTP 认证中间件组合后可覆盖旧 token 拒绝和 cache miss 回源路径。
  - 替代方案：使用真实 Redis 容器。该方案更接近生产，但对当前 token version cache 语义没有明显收益。

- 日志验证使用可观测的 Zap test observer 或内存 sink。
  - 理由：需要断言 request logging 中的 `trace-id`、`user_id`、method、path、status 和 client_ip，而不应依赖真实文件日志或 stdout 文本格式。
  - 替代方案：只断言响应 header，不断言日志字段。该方案无法覆盖本次问题中明确提到的 logging 中间件链。

## Risks / Trade-offs

- 测试耗时增加 -> 优先使用 SQLite、`httptest`、`miniredis`，并将 Docker/PostgreSQL 测试限制在必要场景。
- 集成测试 fixture 过重 -> 将测试 helper 保持在 `_test.go` 文件中，避免引入生产代码或跨包通用 helper 膨胀。
- 403 场景缺少真实授权能力 -> 使用现有响应契约或最小可注入 handler 验证失败信封，不把授权中间件实现纳入本变更。
- Recovery 500 测试可能与业务 controller 错误混淆 -> 分别覆盖业务错误映射 500 和 panic recovery 500，确保响应信封和日志字段都被验证。
- PostgreSQL 容器在部分环境不可用 -> 默认测试不依赖 Docker；若新增容器测试，应通过明确 build tag 或环境跳过策略避免阻塞常规本地测试。
