## Why

`common/security/password` 当前只提供同步 `Hash`/`Verify`，在 Argon2id 高内存成本下缺少请求排队上限、并发保护和 `context.Context` 取消能力。登录、注册或密码校验入口一旦遭遇突发请求，可能放大内存占用并导致 handler goroutine 无界等待。

## What Changes

- **BREAKING**：删除旧的 `Hash` 和 `Verify` 同步入口，统一改为 `HashContext` 和 `VerifyContext`，要求调用方显式传入 `context.Context`。
- 为 `common/security/password` 提供 `HashContext` 和 `VerifyContext`，允许调用方在等待 Argon2id KDF 槽位时通过 context 取消。
- 为 Argon2id 执行增加单进程并发上限和执行中/等待中总队列上限，超出队列容量时返回 `ErrPasswordKDFBusy`。
- 增加明文密码最大长度边界，空密码继续返回 `ErrEmptyPassword`，超长密码返回 `ErrPasswordTooLong`。
- 校验 encoded hash 时继续限制最大长度，并拒绝不符合当前策略的 Argon2id 参数、salt 长度或 key 长度。
- 保持现有 Argon2id hash 输出格式和默认参数兼容，但不保留旧 Go API；不引入业务 HTTP API、配置项、数据库或 Redis 行为变化。
- 实现时评估 `common/security/password` 包是否需要拆分文件；若没有可维护性收益，则直接覆盖现有 `password.go` 并保留测试在同包内。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `common-credentials`: 扩展共享密码凭证原语的安全边界，要求以 context-aware hashing/verifying 替换旧同步入口，并支持 KDF 并发/队列限流、明文长度限制和严格参数策略校验。

## Impact

- 影响代码：`common/security/password/`。
- 影响测试：`common/security/password/password_test.go` 需要覆盖 context 取消、超长密码、参数策略拒绝和 KDF busy 错误语义。
- API 兼容性：这是 Go API 破坏性变更，旧 `Hash`/`Verify` 调用点必须迁移到 `HashContext`/`VerifyContext`。现有 encoded hash 格式、算法版本和默认参数保持兼容。
- 外部系统：不新增配置、数据库、Redis、HTTP route 或响应错误码。
