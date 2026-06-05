## Context

用户服务的 Ent `User` schema 已按领域分类，根 `user-services/ent/schema` package 通过 `user-services/ent/schema/userschema` 暴露实际字段和索引定义。当前 `userschema` 包内存在 schema 本地常量 `defaultUserStatus = 100`，领域层在 `user-services/internal/domain/user_status.go` 中定义 `domain.UserStatusNormal = 100`。两者数值一致，但缺少测试约束与注释说明，未来调整用户状态枚举或持久化默认值时可能发生漂移。

本变更属于 `database-schema-migrations` capability，因为它保护 Ent schema 的数据库默认值语义，并要求不产生新的数据库结构变更。

## Goals / Non-Goals

**Goals:**

- 增加自动化测试，验证 `User` Ent schema 中 `status` 字段默认值等于 `domain.UserStatusNormal` 的数值。
- 在 `defaultUserStatus` 附近补充注释，明确该持久化默认值必须与 `domain.UserStatusNormal` 保持一致。
- 保持 `users.status` 默认值、字段类型、字段注释、索引和约束不变。
- 避免手写 `user-services/ent/` 下的生成代码。

**Non-Goals:**

- 不调整用户状态枚举数值或增加新用户状态。
- 不改变 controller/service/repository 分层、HTTP API、错误映射或响应信封。
- 不生成 Atlas SQL migration，不修改 `user-services/migrations/` 或 `atlas.sum`。
- 不把 Ent schema 重新耦合到 `internal/domain` 包，以免 schema source 引入业务层依赖。

## Decisions

- 在 schema package 测试 Ent 字段描述，而不是测试生成代码。

  这样可以直接覆盖 schema source 中的 `status` 默认值契约，并避免手写或依赖 `user-services/ent/` 生成代码内部结构。备选方案是读取迁移 SQL 或 Ent 生成代码，但这些方式更间接，且容易把一次性实现细节固化为测试。

- 保留 `defaultUserStatus` 作为 schema 本地持久化契约值，并通过测试与注释约束其必须匹配领域正常状态。

  这样可以维持现有的 Ent schema 与 `internal/domain` 解耦设计，避免 Ent codegen/Atlas schema source 因导入业务包而扩大依赖边界。备选方案是直接在 schema 中引用 `domain.UserStatusNormal`，但这会让 schema source 依赖服务内部领域层，不符合当前按 schema source 分类后的边界。

- 测试应位于 `user-services/ent/schema/userschema` 附近，使用 `_test` 外部或同包测试按需要访问字段元数据。

  若需要读取未导出的 `defaultUserStatus`，同包测试更直接；若只验证 Ent 字段默认值，则可不依赖未导出常量。实现时优先选择最小可读方案。

## Risks / Trade-offs

- [Risk] 测试只保护 schema source 的默认值，不能证明既有数据库 migration 文件已同步。→ Mitigation: 本变更不改变默认值数字，不应生成 migration；既有 Atlas 校验流程继续负责 migration directory 一致性。
- [Risk] 直接导入 domain 到 schema 生产代码会造成依赖边界扩大。→ Mitigation: 只允许测试引用 `domain.UserStatusNormal` 做一致性断言，生产 schema 继续使用本地持久化常量。
- [Risk] Ent 字段默认值元数据读取方式可能随 Ent API 有差异。→ Mitigation: 实现时优先使用稳定的字段 descriptor 信息或 schema Fields 返回值进行断言，并运行用户服务测试验证。

## Migration Plan

无需数据库迁移。实现完成后运行 `go test ./...`（至少在 `user-services/` 模块内）验证一致性测试和现有测试通过；`user-services/migrations/` 与 `atlas.sum` 不应发生变化。

## Open Questions

无。
