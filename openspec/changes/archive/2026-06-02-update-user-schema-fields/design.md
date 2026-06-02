## Context

用户服务当前通过 `user-services/ent/schema/user.go` 定义 `User` 数据模型，并由 repository/service/controller 分层完成用户创建和查询。现有 schema 包含 `id`、`name`、`email`、`active`、`created_at`、`updated_at`，其中时间字段为 Ent `field.Time`，DTO 响应也使用 `time.Time`。

本变更涉及显著数据模型调整：新增必填密码字段、将时间字段改为毫秒时间戳、为字段添加数据库 comment，并需要通过 Ent 代码生成与 Atlas migration 落地。`user-services/ent/` 下生成代码不得手写修改。

## Goals / Non-Goals

**Goals:**

- 在 Ent `User` schema 中新增必填 `password` 字段，并在创建用户路径中持久化该字段。
- 将 `created_at`、`updated_at` 改为毫秒级 Unix 时间戳，保持创建和更新默认值自动生成。
- 为用户 schema 每个字段声明稳定、可读的数据库 comment。
- 更新 DTO、service、repository 和测试，使用户查询和创建响应不暴露密码且时间字段为毫秒时间戳。
- 使用 `go generate ./ent` 和 Atlas migration 脚本生成并校验数据库结构变更。

**Non-Goals:**

- 不引入认证、授权、登录、会话或令牌能力。
- 不设计密码哈希算法、密码强度策略或凭据轮换流程；本变更仅按需求持久化必填密码字段。
- 不改变 `common/response.Envelope` 响应契约、错误码体系、配置加载、Redis/PostgreSQL 初始化或 HTTP runtime 启动流程。

## Decisions

- 使用 `field.String("password").NotEmpty()` 表达密码必填约束。理由：与当前 `name`、`email` 的字符串约束保持一致，并让 Ent 生成代码在创建时强制设置该字段。替代方案是允许空值并在 service 校验，但会削弱数据库 schema 约束。
- 使用 `field.Int64("created_at")` 和 `field.Int64("updated_at")` 存储毫秒时间戳，默认值通过 `time.Now().UnixMilli()` 生成，`updated_at` 使用 `UpdateDefault` 自动刷新。理由：请求明确要求时间戳毫秒，`int64` 可直接表达毫秒值并避免时区格式差异。替代方案是在 API 层把 `time.Time` 转为毫秒，但数据库仍保留时间类型，不满足 schema 变更要求。
- DTO 响应中的 `CreatedAt`、`UpdatedAt` 同步改为 `int64`，JSON 字段名保持 `created_at`、`updated_at`。理由：外部可观察语义从 ISO 时间改为毫秒时间戳，DTO 类型应直接反映契约。替代方案是保持 `time.Time` 并自定义 JSON 序列化，但会增加复杂度且不符合最小变更原则。
- `password` 只进入创建请求和 repository 创建输入，不进入 `UserResponse`。理由：密码属于敏感字段，即使当前未实现认证能力，也不应在查询或创建响应中暴露。替代方案是在响应中返回密码用于调试，不符合安全边界。
- 字段 comment 通过 Ent schema 字段链式 `Comment(...)` 声明。理由：schema 是数据库结构事实来源，Atlas 可从 Ent schema source 生成数据库 comment 迁移。替代方案是在 SQL migration 中手写 comment，但会使 schema 与 migration 来源不一致。

## Risks / Trade-offs

- [Risk] 已有 `users` 数据在新增非空 `password` 字段时无法直接迁移。→ Mitigation：审查 Atlas 生成 SQL，必要时添加安全默认值或分阶段回填，并同步更新 `atlas.sum`。
- [Risk] 时间字段类型变更可能破坏依赖 ISO 时间字符串的客户端。→ Mitigation：在 spec 中明确这是外部契约变更，更新测试和 Swagger 示例为毫秒时间戳。
- [Risk] 存储明文密码存在安全风险。→ Mitigation：本变更不引入认证能力，但 implementation 应至少避免在日志和响应中输出密码；后续可单独提出密码哈希/认证能力变更。
- [Risk] 手动编辑 Ent 生成代码会被后续生成覆盖。→ Mitigation：只修改 schema、业务代码和测试，在 `user-services` 模块运行 `go generate ./ent`。

## Migration Plan

1. 修改 `user-services/ent/schema/user.go`，新增 `password` 字段、改造毫秒时间戳字段、补充所有字段 comment。
2. 更新 DTO、service、repository 和测试，确保创建用户时传入密码，响应不包含密码，时间字段使用 `int64`。
3. 在 `user-services/` 模块运行 `go generate ./ent` 生成 Ent 代码。
4. 在 `user-services/` 运行 `./scripts/migrate-diff.sh <name>` 生成 Atlas migration，审查 SQL 中非空密码字段、时间字段类型转换和 comment 语句。
5. 必要时人工调整 migration 以处理已有数据回填，并重新计算/校验 `user-services/migrations/atlas.sum`。
6. 运行 `./scripts/migrate-validate.sh`、`go test ./...` 验证迁移目录和 Go 模块。

Rollback strategy: 若部署前发现问题，回滚代码和未应用 migration；若 migration 已应用，需要通过新的反向 migration 恢复字段类型或删除新增字段，不使用运行时 `client.Schema.Create(ctx)` 回滚。

## Open Questions

- 现有数据迁移时 `password` 的回填值需在实现阶段根据环境策略确认；在未提供策略前，migration SQL 应明确保守处理，避免静默写入可用真实密码。
