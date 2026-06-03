## Context

`user-session-control` 当前在改密成功时由 service 生成新的 Argon2id `password_hash`，再调用 repository 的 `UpdatePasswordHashAndStatus(ctx, userID, passwordHash, status)` 一次性更新 `password_hash`、用户状态并递增 `token_version`。该行为符合会话失效要求，但方法名和参数列表把仓储契约固定为两个字段，后续若凭证更新还需要扩展字段，service 层会继续暴露字段级细节。

本变更只调整 `user-services` 内部认证会话控制边界，不改变 controller、HTTP 路由、响应信封、Redis session 结构、Ent schema 或 Atlas migration。

## Goals / Non-Goals

**Goals:**

- 将用户仓储凭证更新方法重命名为 `UpdateCredentials`。
- 使用 `UpdateCredentialsInput` 结构承载 `UserID`、`PasswordHash` 和目标 `Status`，为凭证更新保留清晰扩展点。
- 保持改密成功后的持久化语义：只更新未软删除用户，设置 `password_hash`，设置目标状态，递增 `token_version`，并返回更新后的 `token_version`。
- 保持 service/repository 分层：service 负责密码哈希和认证编排，repository 负责 Ent 更新和 not found 映射。

**Non-Goals:**

- 不新增或修改 HTTP endpoint、DTO、Swagger 路由语义或响应错误码。
- 不修改 `common/password`、`common/auth`、JWT claims、Redis session key 或 token version cache 策略。
- 不修改 Ent schema、生成代码或数据库 migration。
- 不引入管理员重置密码、用户状态管理、删除用户或授权能力。

## Decisions

- 使用 `UpdateCredentialsInput` 而不是继续追加位置参数。原因是凭证更新字段具有安全语义，命名字段能降低调用方传错参数的风险，也便于后续扩展；备选方案是仅把方法重命名为 `UpdateCredentials(ctx, userID, passwordHash, status)`，但这仍然保留旧参数形态的可读性问题。
- `UpdateCredentials` 继续返回更新后的 `token_version`。原因是该方法承担用户级安全事件的 PostgreSQL 原子递增，调用方和测试可以确认版本变化；备选方案是只返回 `error`，但会削弱安全更新结果的可观察性。
- 保持现有 Ent update 流程：`Where(user.UserIDEQ(input.UserID), user.DeletedAtIsNil())` 后设置凭证字段并 `AddTokenVersion(1)`。原因是该流程已经满足未软删除约束和 token 失效语义；备选方案是先查询再保存，但会增加一次读写窗口且没有必要。
- 不把新的输入结构放入 `common`。原因是它属于 `user-services/internal/repository` 的用户数据访问契约，不是跨服务共享凭证原语；放入 `common` 会扩大模块边界。

## Risks / Trade-offs

- 仓储接口重命名会导致所有测试 stub 和调用点编译失败，直到同步更新。缓解：实现时用 `go test ./...` 在 `user-services` 模块验证接口实现和测试 stub 全部一致。
- 输入结构让未来扩展更容易，但也可能被误用为可选字段集合。缓解：当前实现继续要求 `PasswordHash` 和 `Status` 由 service 明确填充，不引入可选更新语义。
- 该变更不改变数据库结构，因此无法通过 migration 验证发现行为回归。缓解：依赖 repository/service 单元测试覆盖 password hash、状态和 token version 更新语义。

## Migration Plan

- 修改 repository 接口、实现、auth service 调用点和测试 stub 后运行 Go 测试。
- 无数据库迁移、配置迁移或运行时部署顺序要求。
- 回滚时可恢复旧方法名和位置参数；外部 API 与数据结构不受影响。

## Open Questions

- 无。
