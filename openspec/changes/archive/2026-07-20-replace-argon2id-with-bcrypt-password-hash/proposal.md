## Why

当前密码哈希能力使用 Argon2id，并通过服务私有配置暴露 Argon2 并发和队列预算；该实现与后续确定的安全策略不一致，也使认证流程保留了不再需要的 KDF 资源繁忙分支。需要将密码哈希能力收敛到最新 `golang.org/x/crypto/bcrypt`，并明确不再兼容、验证或迁移旧 Argon2id 哈希。

## What Changes

- **BREAKING**：`common/security/password` MUST 删除 Argon2id 哈希、解析、派生和资源门控实现，改为使用 bcrypt 生成和验证密码哈希。
- **BREAKING**：系统 MUST 拒绝旧 Argon2id、未知算法或格式非法的 `password_hash`，MUST NOT 执行旧哈希验证、迁移、fallback 或 rehash。
- **BREAKING**：user-service MUST 删除 `auth.password_kdf.argon2_concurrency` 和 `auth.password_kdf.argon2_queue_size` 配置，不新增 password hash 配置块，bcrypt cost 由共享密码原语固定默认值提供。
- **BREAKING**：认证流程 MUST 删除 `password.ErrPasswordKDFBusy`、`password_kdf_busy` 和相关 `503` 临时不可用分支；密码哈希或校验失败按无效凭据或内部错误的既有边界处理。
- 系统 MUST 使用固定 bcrypt cost，初始值为 `12`。
- 系统 MUST 拒绝超过 bcrypt 安全输入上限的明文密码。
- 用户不存在时的 dummy password verification MUST 使用当前 bcrypt dummy hash，继续避免泄露用户存在性。
- `users.password_hash` 字段语义保持为密码哈希存储，不引入数据库字段或 migration 变更。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`：共享密码安全原语从 Argon2id KDF 实例改为固定策略 bcrypt 哈希服务，并删除 KDF 资源繁忙契约。
- `auth-session-management`：认证登录、改密和配置契约改为使用 bcrypt 密码哈希，不再读取 password KDF 预算配置，也不再返回密码 KDF 繁忙的临时不可用响应。

## Impact

- 影响代码：`common/security/password`、`user-service/internal/config`、`user-service/internal/providers/auth.go`、`user-service/cmd/rbac_dependencies.go`、`user-service/internal/features/auth/application/credentials/verifier.go` 及相关测试。
- 影响配置：删除 `auth.password_kdf.argon2_concurrency` 和 `auth.password_kdf.argon2_queue_size`；旧配置不再被接受。
- 影响数据：`users.password_hash` 字段类型不变；已有 Argon2id 哈希用户发布后无法用旧密码登录，必须通过业务侧重置密码、重新创建账号或重建环境数据恢复访问。
- 影响依赖：继续使用现有 `golang.org/x/crypto v0.54.0`，从该 module 引入 `bcrypt` 包并移除 `argon2` 使用。
- 影响 API：HTTP 路径和 OpenAPI 请求响应结构不变；错误分类中删除 password KDF busy 的 `503 Service Unavailable` 特殊路径。
- 影响观测：登录失败 metrics 中的 `password_kdf_busy` 分类及相关测试需要删除或调整，不新增 bcrypt 运行时资源指标。
- 影响数据库：不修改 Ent schema，不生成 Atlas migration。
