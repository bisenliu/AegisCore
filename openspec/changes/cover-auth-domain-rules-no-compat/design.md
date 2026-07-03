## Context

`user-service/internal/features/auth/domain/credential.go` 定义认证能力消费的最小凭据模型，`CanLogin`、`RequiresPasswordChange` 和 `CanChangePassword` 是登录、强制改密 token 签发和受限改密流程的领域入口。当前主规格已约束普通登录、强制改密登录和 token subject 不得兼容复用，但领域状态判断本身没有直接单元测试覆盖，后续维护容易只在 application 层发现回归。

本变更只补齐 auth domain 测试和对应 OpenSpec delta，不改变生产路径、HTTP API、数据库 schema、OpenAPI 生成物、部署清单或观测资产。

## Goals / Non-Goals

**Goals:**

- 在 `user-service/internal/features/auth/domain` 同包测试中直接覆盖 `UserCredential` 的状态判断。
- 使用 `identity.UserStatusNormal`、`identity.UserStatusDisabled`、`identity.UserStatusMustChangePassword` 和未知状态表达当前稳定语义。
- 使用 `testify/require` 语义化断言，符合 `docs/TESTING.md` 和 auth 测试断言规范。
- 验证覆盖率输出中 `CanLogin`、`RequiresPasswordChange`、`CanChangePassword` 都有直接覆盖。

**Non-Goals:**

- 不修改登录、refresh、logout、强制改密或 HTTP response 行为。
- 不修改 JWT subject、refresh session、token version、password KDF、Redis/PostgreSQL adapter 或 Ent schema。
- 不新增旧状态别名、旧 token 类型、旧错误码、旧字段或兼容判断分支。

## Decisions

- 在 `auth/domain` 包内新增表驱动测试，而不是从 application 或 HTTP 层间接覆盖。理由：这些方法是纯领域判断，直接测试能精确定位状态语义；备选方案是扩展登录用例测试，但会把失败归因混入 password verifier、token issuer 和 session lifecycle mock。
- 将不可登录边界覆盖为 `UserStatusDisabled` 和未知 `identity.UserStatus(0)`。理由：当前 `identity.UserStatus` 只定义 normal、disabled、must change password，未知状态能防止未来无意把默认值当作可登录；备选方案是测试不存在的 deleted/locked 常量，但这会引入旧状态或推测状态兼容语义。
- 只新增测试，不调整 `UserCredential` 实现。理由：现有实现已把普通登录委托给 `identity.UserStatus.CanLogin`，强制改密判断明确匹配 `UserStatusMustChangePassword`；备选方案是抽出额外 helper，但会为测试制造不必要生产代码。

## Risks / Trade-offs

- [风险] 只测试当前存在的身份状态，无法覆盖尚未建模的 locked/deleted 持久化状态。→ [缓解] 使用未知状态作为拒绝边界，并在规格中明确不得引入旧状态或推测状态兼容断言。
- [风险] 覆盖率命令受 Go test cache 影响时不生成新的覆盖文件。→ [缓解] 验证时显式运行 `go test -coverprofile` 并用 `go tool cover -func` 检查目标方法。
- [风险] 工作区已有其他 change 产物和测试改动，整仓 `make verify` 可能被无关 diff 或其他未完成改动影响。→ [缓解] 本变更验证限定到 `auth/domain` 包、`openspec validate cover-auth-domain-rules-no-compat` 和必要的架构 lint；若整仓验证失败，报告与本变更无关的阻塞。

## Migration Plan

本变更无运行时 migration。回滚时移除新增 domain 测试和本 change 的 OpenSpec delta 即可，生产二进制行为不变。

## Open Questions

- 无。
