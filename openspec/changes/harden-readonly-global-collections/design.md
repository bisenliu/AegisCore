## Context

本变更覆盖 `common` 和 `user-service` 中多处只读集合表达。当前代码中的 package-level map、slice 和默认 struct 主要服务于配置校验、metrics label 校验、HTTP metrics label name、scheduler histogram bucket、CORS 默认配置、validation request tag 顺序和 permission HTTP method allowlist。它们在运行时语义上是常量，但 Go 的 map 和 slice 仍可被同包未来代码误写；默认 CORS struct 还可能通过共享 slice 底层数组被调用方间接修改。

受影响路径集中在 `common/runtime/config`、`common/runtime/observability/metrics`、`common/http/middleware`、`common/validation` 和 `user-service/internal/features/permission/domain`。本变更不触碰 `user-service/ent/` 生成代码，不改变数据库 schema、OpenAPI、部署清单、HTTP API 或 RBAC policy sync。

## Goals / Non-Goals

**Goals:**

- 降低非生成代码中 package-level map、slice 和默认 struct 被同包未来代码误写的风险。
- 将只读 map 优先迁移为 `switch`、局部 map、私有查询 helper 或其他不暴露共享可写状态的形式。
- 将只读 slice 优先迁移为数组常量式表达、局部构造或返回副本的函数，避免调用点共享可写底层数组。
- 确保 `CORS()` 和 `CORSWithOptions(defaultCORSOptions)` 的外部行为保持一致，同时默认 CORS 配置不会被调用方通过共享 slice 污染。
- 保持配置校验允许值、弱密钥 denylist、metrics label、scheduler bucket、HTTP method allowlist、错误消息和响应行为不变。

**Non-Goals:**

- 不修改 `user-service/ent/` 生成代码中的 package-level var。
- 不处理 sentinel error、Fx Module、regexp 编译结果、`sync.Pool`、atomic counter 或其他非只读集合类变量。
- 不为形式不可变性引入复杂抽象、反射、泛型容器或新的公开 API。
- 不改变配置文件格式、HTTP API、OpenAPI 生成物、数据库 migration、部署资产或 Prometheus/Grafana 资产。

## Decisions

1. 使用行为等价的私有函数替代只读 map 查询。

   `validLogLevels`、`validLogFormats`、`validPostgresDrivers`、`validPostgresSSLModes`、`validTracingExporters`、`productionLikeEnvironments`、`insecureJWTSecrets` 和 `allowedHTTPMethods` 均属于固定小集合。优先用私有 `switch` helper 表达 membership，避免 package-level map 被误写。备选方案是保留 map 并增加注释，但注释不能阻止写入；也不采用全局 map 每次复制，因为固定小集合用 `switch` 更直接且分配更少。

2. 对 metrics label key 使用不可变查询入口而非共享 map。

   `allowedLowCardinalityLabelKeys` 当前是只读校验集合。实现时优先改为 `switch` 或私有函数，同时保留已有 label key 常量和校验错误语义。备选方案是局部构造 map，但该路径可能在高频校验中产生不必要分配；`switch` 更适合固定低基数标签集合。

3. 对只读 slice 使用复制或数组源。

   `schedulerDurationBuckets`、HTTP metrics label name、`requestTags` 这类有序集合需要保留顺序。实现时可用数组作为源并在需要 slice 的调用点通过 `[:]` 或 `append([]T(nil), source[:]...)` 构造私有副本；如果调用方会把 slice 交给可能保留引用的库，则必须传入新副本。备选方案是保留 package-level slice 并约定不修改，但无法防止同包误写底层数组。

4. 默认 CORS 配置通过函数生成或深拷贝。

   `defaultCORSOptions` 包含多个 slice 字段。实现时应让 `CORS()` 调用私有函数获取新 `CORSOptions`，并在 `CORSWithOptions` 内复制用户传入 options 的 slice 字段后再闭包持有，避免默认值或调用方 options 被后续修改影响 middleware。备选方案是只复制默认值，但不能防止调用方传入 options 后再修改原 slice 造成运行时行为变化。

5. 保留清单只记录真实例外，不扩大实施范围。

   实现阶段应搜索相关 package-level `var`，对不迁移项记录原因。保留理由限于非本类变量或迁移会改变语义的对象，例如 sentinel error、已编译 regexp、Fx Module、`sync.Pool`、atomic counter 和 Prometheus collector 状态。备选方案是全仓机械迁移所有 `var`，风险过高且容易误改有状态对象。

## Risks / Trade-offs

- [Risk] `switch` helper 改写时遗漏允许值或改变错误路径。→ Mitigation：用现有测试和新增表格测试覆盖所有允许值、拒绝值和错误消息。
- [Risk] 复制 slice 后改变 Prometheus descriptor label 顺序或 histogram bucket 顺序。→ Mitigation：保留源顺序并运行 metrics 相关包测试，必要时增加 label name 顺序断言。
- [Risk] CORS options 深拷贝改变调用方依赖“构造后修改 options 生效”的隐式行为。→ Mitigation：该行为不是稳定契约；设计明确 middleware 构造时冻结 options，默认 HTTP 响应头保持不变。
- [Risk] 搜索 package-level var 时误把状态型变量纳入迁移。→ Mitigation：限定到只读 map/slice/default struct，并在任务中列出保留项及理由。
- [Risk] common 与 user-service 分属不同 Go module，验证命令遗漏。→ Mitigation：分别在 `common` 和 `user-service` 模块运行目标测试。

## Migration Plan

本变更是内部实现重构，无数据迁移、配置迁移、HTTP API 迁移或部署顺序要求。发布时可按普通代码变更随服务镜像滚动上线；如需回滚，回退本 change 的代码提交即可，不需要数据库、OpenAPI 或部署资产回滚。

实现验证包括：在 `common` 模块运行 `go test ./runtime/config ./runtime/observability/metrics ./http/middleware ./validation`；在 `user-service` 模块运行覆盖 permission HTTP method/domain 校验的相关测试；如修改文档或规格，运行 `make user-service-architecture-lint`。

## Open Questions

无。
