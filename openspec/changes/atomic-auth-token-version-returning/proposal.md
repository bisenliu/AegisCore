## Why

当前 auth 的 `IncrementTokenVersion` 和 `UpdateCredentials` 先提交 PostgreSQL `UPDATE`，再通过第二条 `SELECT` 读取新 `token_version`。当第二次读取因 context 取消、连接中断、查询超时或数据库瞬时错误失败时，调用方会看到失败，但密码或 `token_version` 已经改变，后续 Redis 投影刷新和 refresh session 撤销会被跳过。

这是认证撤销链路的 P0 安全一致性缺口。需要在实现前明确规格：安全状态 mutation 与新撤销版本必须由单一确定的数据库结果返回，不能存在提交后才获取版本的失败窗口。

## What Changes

- 修改 auth 凭证和 token version 持久化语义：`token_version` 递增、密码哈希更新、状态更新与新 `token_version` 返回必须形成单一确定结果。
- `IncrementTokenVersion` 的成功路径必须通过 PostgreSQL `UPDATE ... RETURNING token_version` 或等价事务内更新返回取得新版本，不得在提交后再执行第二条 `SELECT` 获取新版本。
- `UpdateCredentials` 的成功路径必须在同一数据库结果中返回新版本；条件不匹配继续返回统一无效凭据，用户不存在继续返回用户不存在。
- 已更新 PostgreSQL 主事实且拿到新 `token_version` 后，调用方必须进入可恢复撤销编排，执行本地缓存失效、Redis 投影刷新和 refresh session 撤销。
- 撤销投影失败必须继续以 `authdomain.ErrSessionRevocationIncomplete` 暴露，不能被混同为凭证或 token version 更新失败。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `auth-session-management`: 收紧 token version 主事实递增、强制改密凭证更新和退出全部会话撤销的原子返回与撤销编排要求。

## Impact

- 影响 `user-service/internal/features/auth/infrastructure/postgres/credential_store.go` 中 `IncrementTokenVersion` 和 `UpdateCredentials` 的 PostgreSQL 更新实现。
- 影响 auth application 撤销链路的验收标准，包括 `sessions.RevokeAllUserSessions`、强制改密用例和 token version Redis 投影刷新。
- 不变更 HTTP API 路径、请求体或响应 schema；失败分类仍使用既有认证错误与 `authdomain.ErrSessionRevocationIncomplete`。
- 不引入数据库 schema 变更或 Atlas migration。
- 需要补充 PostgreSQL adapter 和 auth application 单元测试，覆盖提交后第二次读取窗口被移除、条件更新、用户不存在和撤销投影不完整路径。
