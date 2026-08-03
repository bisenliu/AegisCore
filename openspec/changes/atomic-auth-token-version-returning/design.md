## Context

auth 的退出全部会话和强制改密依赖 PostgreSQL `users.token_version` 作为撤销主事实。当前 PostgreSQL adapter 在 `UPDATE` 成功提交后再通过第二条 `SELECT` 读取新版本，导致“主事实已改变但调用方未拿到新版本”的不确定窗口。该窗口会阻断后续本地缓存失效、Redis 投影刷新和 refresh session 撤销，属于认证安全边界问题。

本次变更只调整 user-service auth 内部持久化和撤销编排验收，不改变 HTTP API、OpenAPI、数据库 schema、部署清单或 common 共享模块。

## Goals / Non-Goals

**Goals:**

- 让 `IncrementTokenVersion` 在 Ent 事务内完成递增并返回新 `token_version`。
- 让 `UpdateCredentials` 在同一确定数据库结果中完成密码哈希、状态和 `token_version` 更新，并返回新版本。
- 保持用户不存在、条件不匹配和撤销投影不完整的既有错误分类。
- 确保成功更新 PostgreSQL 主事实后，调用方进入 `RevokeUserSessionsAtVersion`，后续失败按撤销投影不完整处理。
- 通过测试证明成功路径不存在提交后的第二次 `SELECT`。

**Non-Goals:**

- 不新增兼容分支，不保留旧的 `UPDATE` 后 `GetTokenVersion` 成功路径。
- 不修改 users 表结构、Ent schema 或 Atlas migration。
- 不改变 JWT claims、Redis key schema、HTTP 路径、响应体或 OpenAPI 生成物。
- 不把 auth 业务 DTO、撤销编排或 PostgreSQL SQL helper 上移到 `common` 或 `internal/shared`。

## Decisions

- 使用 Ent `UpdateOneID(...).Select(token_version).Save(ctx)` 作为成功更新路径。
  - 理由：Ent 生成的单实体更新会开启事务，执行更新后在同一事务内回填实体字段，消除提交后第二次读取窗口，同时保留 schema 字段名、默认 `updated_at` 和 adapter 现有 Ent 边界。
  - 备选方案：手写 PostgreSQL `UPDATE ... RETURNING token_version`。该方案也能满足原子返回，但会增加 auth adapter 对原生 SQL 连接池和手写列名的依赖，本次不采用。

- `IncrementTokenVersion` 直接按 `user_id` 和 `deleted_at IS NULL` 更新并返回新版本。
  - 理由：退出全部会话只需要判断用户是否存在并取得递增后的撤销版本。
  - 备选方案：保留 Ent `Update().Save()` 再读取。该方案被明确移除，因为它就是故障窗口根因。

- `UpdateCredentials` 使用条件更新并返回新版本，条件不匹配时区分用户不存在和 token/status 不匹配。
  - 理由：强制改密需要在 `ExpectedStatus` 和 `ExpectedTokenVersion` 同时匹配时才改变密码、状态和版本；不匹配必须映射为统一无效凭据。
  - 备选方案：先查用户再更新。该方案会重新引入竞态或要求更复杂事务；本次选择单条 SQL 或同一语句 CTE 完成判断。

- 继续将撤销投影作为 PostgreSQL 更新成功后的 application 编排。
  - 理由：Redis 投影和 refresh session 是可恢复的副作用，现有代码已经用 `ErrSessionRevocationIncomplete` 表达“主事实已更新但投影不完整”。本次不引入 outbox、eventbus 或后台补偿机制。
  - 备选方案：把 Redis 删除纳入同一事务。PostgreSQL 与 Redis 无法共享原子事务，该方案不可行且会扩大故障面。

- 测试聚焦 adapter 行为和 use case 编排，不新增生产专用测试分支。
  - 理由：应通过真实 PostgreSQL/sqlmock 可观察 SQL、错误注入和现有 mock port 验证行为，不为了测试添加无业务意义的接口层。

## Risks / Trade-offs

- [Risk] Ent 单实体更新需要先定位内部自增 ID，定位和更新之间用户可能被软删除。
  → Mitigation：更新语句继续携带 `deleted_at IS NULL` 谓词；若定位后条件失效，Ent 返回 not found 并映射为用户不存在或条件不匹配。

- [Risk] `UpdateCredentials` 条件不匹配和用户不存在的 SQL 返回分支处理错误会改变公开错误分类。
  → Mitigation：测试分别覆盖用户不存在、状态不匹配、版本不匹配和成功更新。

- [Risk] context 在 `UPDATE ... RETURNING` 执行期间取消时，客户端仍可能无法确定数据库是否提交。
  → Mitigation：规格和实现只保证服务端成功路径不存在“已提交但无版本”的第二步窗口；数据库驱动在单条语句返回错误时不会让 application 进入撤销成功路径，调用方按未确认失败处理。

- [Risk] Redis 投影或 session 删除失败仍可能导致撤销不完整。
  → Mitigation：保持现有 `ErrSessionRevocationIncomplete`、日志和 metrics 语义，明确该错误表示 PostgreSQL 主事实已更新但副作用未完成。

## Migration Plan

1. 更新 `CredentialStore.IncrementTokenVersion` 和 `CredentialStore.UpdateCredentials` 的 Ent 更新实现，移除成功路径中的 `GetTokenVersion` 调用。
2. 补充或调整 PostgreSQL adapter 测试，验证成功路径返回新版本、条件失败不更新、用户不存在不更新。
3. 补充 auth application 测试，验证拿到更新版本后进入撤销编排，投影失败返回 `ErrSessionRevocationIncomplete`。
4. 运行相关 user-service auth 测试和 `make user-service-architecture-lint`。
5. 不需要数据库迁移或部署顺序变更；回滚为回退应用代码版本。

## Open Questions

无。
