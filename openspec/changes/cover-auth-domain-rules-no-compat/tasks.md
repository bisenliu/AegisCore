## 1. 领域测试实现

- [x] 1.1 在 `user-service/internal/features/auth/domain` 中新增或补充 `UserCredential` 状态判断测试，直接覆盖 `CanLogin`、`RequiresPasswordChange` 和 `CanChangePassword`。
- [x] 1.2 使用当前 `identity.UserStatusNormal`、`identity.UserStatusDisabled`、`identity.UserStatusMustChangePassword` 和未知状态值构造表驱动用例，不新增旧状态别名、旧 token 类型或兼容 helper。
- [x] 1.3 使用 `testify/require` 语义化断言表达布尔和值预期，避免机械 `Fail` / `Failf` 或旧手写断言。

## 2. 规格和任务同步

- [x] 2.1 确认 `openspec/changes/cover-auth-domain-rules-no-compat/specs/auth-session-management/spec.md` 覆盖普通登录、强制改密、不可登录状态和语义化断言要求。
- [x] 2.2 实现完成后更新本清单，把实际完成的任务改为 `- [x]`。

## 3. 验证

- [x] 3.1 运行 `go test -coverprofile=/tmp/cover-auth-domain-rules-no-compat.out ./user-service/internal/features/auth/domain`，确认 auth domain 包测试通过且覆盖率不再为 0%。
- [x] 3.2 运行 `go tool cover -func=/tmp/cover-auth-domain-rules-no-compat.out`，确认 `CanLogin`、`RequiresPasswordChange`、`CanChangePassword` 均有直接覆盖。
- [x] 3.3 运行 `openspec validate cover-auth-domain-rules-no-compat`。
- [x] 3.4 在不混入无关工作区改动的前提下暂存本次预期代码和 OpenSpec 产物，并运行 `make lint` 和 `make verify`；如果被其他未完成 change 或非本次文件阻塞，记录原因，不把该项标为完成。
