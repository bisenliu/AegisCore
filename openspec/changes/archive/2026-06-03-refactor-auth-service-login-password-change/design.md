## Context

`user-session-control` 已经定义登录、Refresh Token 会话、退出和强制改密的外部行为。当前 `authService.Login()` 同时完成请求值归一化、用户查询、密码校验、状态判断和 token 签发，`ChangePassword()` 同时完成新密码校验、改密 token 解析、token version 校验、用户查询、状态校验和凭证更新。实现能够工作，但职责边界不够清晰，后续调整错误映射、测试认证失败路径或复用 token 校验逻辑时容易产生回归。

本次变更只作用于 `user-services/internal/service/auth_service.go` 的 service 层内部结构。controller 仍负责 HTTP 绑定和响应输出，repository 仍负责 Ent/PostgreSQL 访问，`common/auth` 和 `common/password` 仍作为 JWT 与密码凭证原语来源。

## Goals / Non-Goals

**Goals:**

- 让 `Login()` 聚焦认证成功后的签发策略，避免把用户认证细节和签发分支混在同一方法内。
- 让用户认证 helper 集中处理空 username/password、用户不存在、密码校验失败和禁用状态拒绝，并统一映射为现有凭据无效响应。
- 让改密 token helper 聚焦 token 归一化、解析、token version 校验和 `user_id` UUID 解析，返回 `uuid.UUID` 给 `ChangePassword()` 继续执行业务状态读取和凭证更新。
- 保持所有外部可观察行为兼容，包括 HTTP 响应契约、错误分类、token subject、TTL、Redis 会话写入和数据库写入。

**Non-Goals:**

- 不新增登录、改密、刷新或退出 API。
- 不修改 controller、router、DTO、repository 接口、Ent schema、migration 或配置结构。
- 不改变 `status=300` 用户的强制改密语义，也不把该状态视为认证失败。
- 不引入新的公共 service 接口或跨模块共享 helper。

## Decisions

- 在 `authService` 内新增私有 `authenticateUser(ctx, username, plainPassword string) (*ent.User, error)` helper。
  - 理由：该 helper 属于用户服务认证编排的一部分，需要访问 repository、日志和密码校验，放在 service 私有方法可避免扩大公共接口。
  - 替代方案：把认证逻辑放入 repository。放弃该方案，因为密码校验和错误映射属于 service 编排，不应进入数据访问层。

- `authenticateUser()` 不处理 `UserStatusMustChangePassword` 的 token 签发分支。
  - 理由：`status=300` 表示用户凭据认证成功后的签发策略差异，不是认证失败；保留在 `Login()` 可以让普通 token pair 与受限改密凭据的分支更直观。
  - 替代方案：helper 直接返回 token 响应。放弃该方案，因为会重新把认证和签发耦合在一起。

- 在 `authService` 内新增私有 `verifyPasswordChangeToken(ctx, token string) (uuid.UUID, error)` helper。
  - 理由：该 helper 的职责边界精确到凭据验证和身份解析；返回 UUID 后，`ChangePassword()` 继续负责用户查询、状态校验、密码 hash、凭证更新和缓存失效。
  - 替代方案：helper 直接返回 `*ent.User`。放弃该方案，因为会让 token 验证同时承担用户状态读取，职责过宽。

- 保持现有错误映射和日志语义。
  - 理由：本次变更是内部结构调整，不应改变 API 调用方看到的认证失败、token 无效、用户不存在或校验失败响应。
  - 替代方案：借机调整错误码或文案。放弃该方案，因为会扩大变更范围并破坏兼容性。

## Risks / Trade-offs

- [Risk] helper 拆分可能在重排逻辑时改变 `status=300` 或禁用用户的分支顺序 -> 通过保持 `Login()` 显式先处理 `UserStatusMustChangePassword`、再处理普通登录资格，并运行用户服务测试缓解。
- [Risk] 改密 token helper 返回 UUID 后，日志中仍需要字符串 user id -> 在 `ChangePassword()` 中使用 `parsedUserID.String()` 或保留 claims user id 的等价字符串，避免记录 token 原文。
- [Risk] 只做内部重构可能没有新增外部行为测试 -> 优先补充或维护现有 auth service 单元测试覆盖无效凭据、禁用用户、强制改密和改密 token 无效路径。

## Migration Plan

无需数据迁移、配置迁移或部署编排变化。实现可作为普通 Go 代码变更部署；如发现回归，可回滚到变更前的 service 方法结构，不涉及 Redis key、JWT 格式或数据库 schema 回滚。

## Open Questions

无。
