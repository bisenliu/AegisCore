## Context

当前 user-service 已具备 HTTP metrics、SQL 连接池 metrics、Redis PING metrics、localcache metrics、Gin tracing 和结构化日志，但 RBAC Enforce、Redis 命令和 Ent 查询仍缺少热路径级别的低基数指标与 trace span。

RBAC 授权路径位于 `user-service/internal/features/permission/application/authorization` 与 `user-service/internal/features/permission/infrastructure/casbin`；Redis client 由 `common/runtime/datastore` 创建并在 user-service provider 中作为具名资源注入；Ent client 由 `user-service/internal/providers/ent.go` 基于具名 `*sql.DB` 包装创建。

本变更跨越 `common` runtime primitive、user-service permission feature、user-service provider 和部署观测资产。实现必须保持业务边界：RBAC 业务指标留在 permission feature，Redis OTel hook 可放在 common datastore，Ent query 观测留在 user-service provider 或服务级观测代码，不把 user-service 业务语义下沉到 common。

## Goals / Non-Goals

**Goals:**

- 为每次 RBAC Enforce 判定记录 `allow`、`deny`、`error` counter 和 latency histogram。
- 确保 RBAC Enforce 指标只使用 `result`、`method`、`route_template` 标签，不包含用户 ID、角色 ID、权限 ID、raw path 或原始错误。
- 为 `common/runtime/datastore` 创建的 go-redis client 安装 OpenTelemetry hook，使 Redis 命令在已有 trace context 下产生 client span。
- 为 user-service Ent 查询增加 query span、query latency histogram 和 query error counter。
- 更新部署观测资产和验证脚本，使新增指标可被 Prometheus/Grafana/alerts 或 metrics load 验证消费。
- 不保留旧无观测路径、旧指标名、旧标签或兼容 PromQL。

**Non-Goals:**

- 不改变 RBAC 授权语义、Casbin policy 权威来源、policy sync、用户角色缓存失效或超级管理员通配授权。
- 不新增 HTTP API、OpenAPI 字段、Ent schema 或 Atlas migration。
- 不记录 raw SQL、SQL 参数、Redis key、用户 ID、角色 ID、权限 ID、token、trace/span ID 或原始错误到 metrics label。
- 不为 Redis key schema、业务缓存 key 或 Ent 业务 DTO 设计新的共享 abstraction。

## Decisions

1. RBAC Enforce 指标放在 permission feature 边界内。

   `authorization.service.Enforce` 是最合适的埋点入口，因为它同时看到认证 subject 解析结果、`method`、`route_template` 和底层 engine 返回结果。指标接口应扩展 feature-local metrics，而不是放入 `common/runtime/observability/metrics`，避免 common 承载 user-service RBAC 业务语义。

   不采用在 `common/security/casbin.Enforce` 中记录指标，因为该包只应承载无业务语义的 Casbin 三元组 helper，无法稳定表达 user-service route template 和业务结果边界。

2. RBAC Enforce 指标使用固定结果枚举和固定标签。

   counter 建议命名为 `aegiscore_user_service_rbac_enforce_total`，histogram 建议命名为 `aegiscore_user_service_rbac_enforce_duration_seconds`。标签只允许 `result`、`method`、`route_template`，其中 `result` 固定为 `allow`、`deny`、`error`。

   不保留 `user_id`、role、permission、raw path 或 error label，也不额外提供旧 label 或兼容 PromQL。

3. Redis OpenTelemetry hook 在 common datastore 创建点统一安装。

   `common/runtime/datastore.OpenRedisClient` 是共享 Redis client 构造入口，在这里安装 hook 可以覆盖 user-service cache Redis、后续服务复用和生命周期 ping，同时避免每个服务 provider 重复安装。实现应引入 go-redis 官方 extra instrumentation，并在 client 创建后立即安装。

   不采用在 user-service provider 私有安装 hook，因为这会遗漏其他共享 datastore 调用方并形成服务侧重复逻辑。

4. Ent query 观测放在 user-service Ent client provider。

   Ent client 是 user-service 的生成代码和服务级 DB 访问入口，query span 与 query metrics 应在 `user-service/internal/providers/ent.go` 创建 client 后统一安装。query span 应使用服务 tracer，query metrics 应使用服务 metrics provider，标签使用稳定低基数 entity/query/result 枚举。

   不采用在 Ent 生成代码中手写修改；生成目录不得手写。也不采用在 `common/runtime/datastore` 记录 Ent 语义，因为 common 只持有 `database/sql` 连接池，不拥有 Ent entity 或 query 类型。

5. 部署观测资产直接消费新指标。

   Prometheus alert、Grafana dashboard 或 metrics load 验证应直接引用新增 metric family。由于现有系统没有 RBAC Enforce 或 Ent query 指标，变更不提供旧指标别名，也不保留兼容 PromQL。

## Risks / Trade-offs

- Redis instrumentation 依赖版本不匹配 -> 通过 `go test ./...`、相关 provider 测试和最小 Redis span 测试验证 go-redis v9 hook 与当前 OTel 版本兼容。
- RBAC Enforce histogram 按 route template 增加序列数量 -> 只使用 Gin route template，不使用 raw path，并通过测试断言 label key/value 受限。
- Ent query entity 标签过细或不稳定 -> 使用显式映射或稳定归一化函数，将未知类型折叠为 `unknown`，不直接把完整 Go 类型或 SQL 作为 label。
- tracing 禁用时引入额外开销 -> 使用现有 tracing provider/no-op tracer 行为，metrics 禁用时返回 no-op recorder，不改变业务返回。
- 部署观测资产 drift -> 更新 dashboard/alert/load 验证后运行对应检查命令，避免 PromQL 引用不存在的 metric family。

## Migration Plan

1. 扩展 permission feature metrics interface 和 no-op 生成，使 RBAC Enforce 可记录 result、method、route_template、duration。
2. 在 permission metrics provider 注册 RBAC Enforce counter 和 histogram，并在授权服务 Enforce 入口记录 allow、deny、error。
3. 在 common Redis client 创建点安装 go-redis OpenTelemetry hook，并补充 tracing 测试。
4. 在 user-service Ent client provider 安装 query tracing 和 query metrics，不修改 Ent 生成代码。
5. 更新 Prometheus/Grafana/metrics load 验证资产，直接消费新增指标。
6. 运行相关包测试、`make user-service-architecture-lint`、观测资产检查；合并前运行 `make lint` 和 `make verify`。

回滚方式：回滚本 change 对代码、依赖和部署观测资产的提交；由于不涉及数据库 schema、HTTP API 或持久化数据，无需数据迁移回滚。

## Open Questions

无。
