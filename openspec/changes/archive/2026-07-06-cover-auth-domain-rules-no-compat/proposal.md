## Why

`auth/domain` 中 `UserCredential` 的状态判断直接决定普通登录、强制改密登录和受限改密流程能否继续，但当前这些领域判断缺少直接单元测试覆盖。补齐测试可以把认证状态语义固定在领域模型层，避免后续通过旧状态别名、旧 token 类型或兼容错误语义绕开现有安全约束。

## What Changes

- 为 `user-service/internal/features/auth/domain` 新增领域单元测试，直接覆盖 `UserCredential.CanLogin`、`UserCredential.RequiresPasswordChange` 和 `UserCredential.CanChangePassword`。
- 测试覆盖正常、停用、强制改密以及未知或不可登录状态，确认仅当前身份状态语义可触发普通登录或受限改密流程。
- 测试断言遵循 `docs/TESTING.md` 和既有 auth 测试语义化断言规范，常规断言使用 `testify/require`。
- **BREAKING**：测试不得引入旧状态别名、旧 token subject 复用、旧错误码或旧字段的兼容断言路径。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`：补充认证领域状态判断测试要求，固定 `UserCredential` 对普通登录、强制改密和改密流程的当前状态语义。

## Impact

- 影响代码：`user-service/internal/features/auth/domain/*_test.go`，必要时只读取 `credential.go`、`session.go`、`errors.go` 的现有领域行为。
- 影响规格：新增 `openspec/changes/cover-auth-domain-rules-no-compat/specs/auth-session-management/spec.md`。
- 不影响 HTTP API、OpenAPI、数据库 schema、JWT/refresh session、token version、password KDF、Redis 存储或部署资产。
