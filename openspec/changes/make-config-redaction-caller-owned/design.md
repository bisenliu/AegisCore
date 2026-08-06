## Context

当前 `common/runtime/config.RedactSettings(settings, nil)` 会使用 common 内置默认路径，其中包含 `auth.jwt.secret`、`resources.redis.*.password` 和 `resources.postgres.*.password`。这些路径中 `auth.jwt.secret` 是 user-service 私有 auth schema，Redis 与 PostgreSQL 资源路径虽然跨服务可复用，但具体是否敏感、是否存在以及是否需要新增字段仍应由消费服务声明。

本次变更横跨 `common` 与 `user-service`：`common/runtime/config` 只保留 deep clone、路径通配匹配和 YAML render primitive；`user-service` 在 CLI effective render 边界集中声明自身敏感路径，并显式传给 `RedactSettings`。OpenSpec 同步更新 `shared-platform-primitives` 与 `auth-session-management`，防止后续在 common 中重新加入服务私有 secret schema。

## Goals / Non-Goals

**Goals:**

- 移除 `common/runtime/config` 生产代码与测试对 `auth.jwt.secret` 或其他 feature 私有路径的认知。
- 保持 `RedactSettings` 对输入 settings 的不变性，支持 nil settings、空路径、未知路径、通配 map、多层 slice/map 和调用方声明路径。
- 让 user-service render 输出继续脱敏 JWT、Redis、PostgreSQL 及服务私有敏感字段。
- 使未来新服务只需在调用方声明敏感路径即可复用 common redaction primitive。

**Non-Goals:**

- 不改变 YAML deep merge、strict decode、raw digest、effective settings encode 或配置来源加载流程。
- 不改变 Nacos dataId、环境变量名、运行时配置结构或资源 schema。
- 不在 common 中通过字段名字符串猜测、反射 tag 或自动发现机制推断敏感字段。
- 不新增数据库 migration、OpenAPI 输出、HTTP API、部署清单或观测资产。

## Decisions

- `RedactSettings` 的 nil 路径语义改为“无内置默认路径”。理由：nil 不应隐式携带 user-service 策略，调用方必须显式传入完整策略。备选方案是保留 Redis/PostgreSQL 默认路径并只移除 `auth.jwt.secret`，但这仍会让 common 对具体配置布局承担策略所有权。
- 在 user-service 增加集中敏感路径列表和 render helper。理由：CLI 与测试都应复用同一服务策略，避免多个调用点手写路径。备选方案是在 CLI 内联路径 slice，但后续新增服务私有 secret 时容易遗漏测试或其他 render 调用方。
- 通配匹配继续使用点分路径和 `*` 匹配 map key，并扩展到 slice 内元素。理由：具名资源和列表型配置都需要同一规则表达，且不引入 schema 反射。备选方案是支持更复杂的 JSONPath，但会扩大 common API 面和测试矩阵。
- 脱敏仍返回 deep copy，不修改输入 map/slice。理由：effective settings 还可能用于 digest、调试或后续渲染，不应被 render 副作用污染。备选方案是原地脱敏以减少分配，但会破坏当前输入不变约束。

## Risks / Trade-offs

- 调用方忘记传入敏感路径可能导致 render 泄漏 secret → user-service 提供单一 helper，并添加 render 不包含真实 secret 的测试。
- nil 路径语义变化可能影响未来未知调用点 → 全仓搜索 `RedactSettings(`，将需要脱敏的调用点改为服务显式策略；common 测试覆盖 nil 与空路径均不脱敏。
- slice 通配递归扩大匹配范围，可能脱敏调用方未预期的同名字段 → 仅在调用方显式路径命中时处理，未知路径保持 no-op。
- 删除 common 默认路径会降低“开箱即用”便利性 → 用更清晰的所有权边界换取跨服务复用正确性。

## Migration Plan

- 更新 common redaction primitive，删除 `defaultRedactPaths` 和所有 user-service 私有路径。
- 在 user-service config 包声明 `SensitiveConfigPaths` 或等价 helper，并由 `cmd config render` 使用。
- 更新 user-service 配置测试改用服务私有 helper，更新 common redaction 测试不再断言 auth 路径默认脱敏。
- 运行相关 config 测试、CLI 测试、`make user-service-architecture-lint`、`make lint` 和 `make verify`。
- 回滚方式：恢复 common 默认路径与旧调用方式；无需数据迁移或部署资源回滚。

## Open Questions

- 无待决问题；敏感路径集合以当前 user-service JWT、Redis、PostgreSQL 和服务私有配置结构为准，由实现阶段全仓搜索确认最终列表。
