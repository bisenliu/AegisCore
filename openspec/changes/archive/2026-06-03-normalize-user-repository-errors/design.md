## Context

当前用户资料查询相关 repository 路径已经把 Ent not found 转换为 `domain.ErrUserNotFound`，再由 service 层映射为 `common/response` 应用错误。但用户会话控制相关的 `GetTokenVersion`、`IncrementTokenVersion` 和 `UpdateCredentials` 仍在 PostgreSQL repository 实现中直接构造 `response.NotFoundError`，使数据访问层依赖 HTTP 响应契约。

该变更只调整错误边界，不改变 controller、路由、HTTP envelope、错误码、Redis 会话语义、PostgreSQL schema 或 Ent 生成代码。

## Goals / Non-Goals

**Goals:**

- 让 `user-services/internal/repository/postgres` 中的用户不存在路径统一返回 `domain.ErrUserNotFound`。
- 保持 service 层集中负责将领域错误映射为 `response.NotFoundError(errmsg.MsgUserNotFound)` 或既有会话控制错误响应。
- 通过测试覆盖 token version 读取、token version 递增和凭据更新的用户不存在路径。
- 保持外部 API 的 HTTP status、业务 code、message 和 envelope 结构不变。

**Non-Goals:**

- 不新增认证、会话或用户资料 API。
- 不修改 Redis key、Refresh Token 轮转、token version 缓存或认证中间件行为。
- 不修改 Ent schema、生成代码、Atlas migration 或数据库结构。
- 不迁移 `common/response` 的构造函数或错误码定义。

## Decisions

1. Repository 只返回领域错误或底层错误。

   `GetTokenVersion`、`IncrementTokenVersion` 和 `UpdateCredentials` 在未匹配到未软删除用户时返回 `domain.ErrUserNotFound`。这样与 `Create`、`GetByUserID`、`GetByUsername` 保持同一 repository 错误风格，并避免 `repository/postgres` 继续依赖响应层错误构造职责。

2. Service 保留应用错误映射职责。

   service 层继续使用 `errors.Is(err, domain.ErrUserNotFound)` 判断业务含义，并按现有流程映射为凭据无效、not found、token invalid 或内部错误。这样不会改变对外响应契约，也不需要 controller 感知 repository 实现细节。

3. 不为该变更引入兼容分支。

   当前错误对象未作为持久化数据或外部 API 暴露，内部调用方可以统一迁移到领域错误判断，不需要同时支持 repository 返回 `response.AppError` 的旧路径。

## Risks / Trade-offs

- [Risk] 某个 service 路径只识别 `response.AppError` 而未识别 `domain.ErrUserNotFound`，可能导致 404/认证失败变成 500。→ Mitigation：实现时检查会话控制 service 中三个方法的调用路径，并补充用户不存在场景测试。
- [Risk] Ent update one 路径和查询路径的 not found 错误形态不同，可能遗漏转换。→ Mitigation：分别覆盖 `ent.IsNotFound` 和零更新/未找到路径，确保全部转换为 `domain.ErrUserNotFound`。
- [Risk] 过度扩大范围会触碰认证响应文案或 Redis 会话逻辑。→ Mitigation：只改 `repository/postgres` 的错误返回和必要 service 测试，不改变 API、Redis key 或 token 签发逻辑。
