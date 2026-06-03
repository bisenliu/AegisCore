## Context

`common/credentials` 当前聚合了三类共享原语：密码 Argon2id hash/verify、JWT token 签发解析、认证传输常量与 context helper。该聚合包已经被 `common/middleware`、`user-services/internal/bootstrap`、`user-services/internal/router`、`user-services/internal/service`、`user-services/internal/controller` 和相关测试直接导入。

本变更是 common 模块内的包边界重构，不改变运行时 HTTP API、JWT 载荷、密码 hash 格式、配置结构、Redis/PostgreSQL/Ent 依赖或响应信封。实现时需要同时迁移 common 与 user-services 两个 Go module 的导入路径，确保 workspace 全量测试继续通过。

## Goals / Non-Goals

**Goals:**

- 删除 `common/credentials` 目录，将密码能力移动到 `common/password`，认证/JWT 能力移动到 `common/auth`。
- 将密码公开 API 调整为 `password.Hash()` 和 `password.Verify()`，保留现有错误语义和 hash 格式。
- 将认证上下文 helper、Authorization/Bearer 常量、JWT service、claims、subject 和 sign input 统一放入 `common/auth`。
- 更新所有现有调用方和测试，不保留旧 `credentials` 包兼容层。
- 保持外部可观察行为不变，包括 HTTP 状态码、业务错误码、响应 envelope、token claims 和配置字段。

**Non-Goals:**

- 不新增认证、授权、会话或密码策略功能。
- 不修改 Ent schema、Atlas migration、数据库数据或 Redis 会话 key 语义。
- 不修改 YAML 配置结构、环境变量命名或配置加载策略。
- 不调整 controller/service/repository 分层职责。
- 不为旧 `github.com/aegiscore/common/credentials` 包路径提供迁移适配器。

## Decisions

- 使用两个独立包表达职责边界：`common/password` 只包含密码 hash/verify，`common/auth` 包含 JWT、认证常量和认证 context。这样比在 `common/credentials` 中继续增加子文件更清晰，也避免业务认证代码导入密码 helper 时暴露不相关 API。
- 密码 API 改名为 `Hash` 与 `Verify`，而不是继续保留 `HashPassword` 与 `VerifyPassword`。包名已经表达 password 语义，短函数名减少重复；旧函数名不保留，符合本次明确的 breaking package cleanup。
- JWT service 类型和方法保持语义不变，只迁移包名。`JWTService`、`Claims`、`SignInput`、`SubjectAccess`、`SubjectRefresh`、`SubjectPasswordChange` 和错误变量继续作为 `common/auth` 的公开契约，避免认证中间件和用户会话业务复制 token 规则。
- 认证传输常量和 context helper 与 JWT 放在 `common/auth`，而不是拆成第三个 transport/context 包。它们共同描述认证边界，调用方通常在中间件和会话逻辑中一起使用；继续保持一个认证包能降低导入复杂度。
- 不新增兼容 re-export 包。旧 `common/credentials` 目录会被删除，所有仓库内调用点必须一次性迁移；这能防止新代码继续依赖旧包路径。

## Risks / Trade-offs

- 破坏旧包路径和密码函数名 -> 通过全仓搜索 `common/credentials`、`credentials.`、`HashPassword`、`VerifyPassword` 并运行 common 与 user-services 测试来降低遗漏风险。
- 拆包后同一文件可能同时需要 `auth` 与 `password` 导入 -> 只在确实同时签发 token 与校验密码的 service 中双导入，避免重新引入聚合包。
- JWT 行为在迁移中被意外改动 -> 迁移时优先移动代码并只改包名，保留现有 JWT 单元测试覆盖 issuer/audience、subject、identity fields 和 missing secret。
- 密码 hash 行为在函数改名中被意外改动 -> 保留现有测试用例并更新断言调用到 `password.Hash`/`password.Verify`。

## Migration Plan

- 创建 `common/password` 与 `common/auth` 目录，移动对应源码和测试并更新 package 名称。
- 删除 `common/credentials` 目录。
- 更新 common 模块调用方：`common/middleware/auth.go`、`common/middleware/cors.go` 和测试导入 `common/auth`。
- 更新 user-services 模块调用方：bootstrap、router、controller、service 和测试根据用途导入 `common/auth` 或 `common/password`。
- 在 `common/` 和 `user-services/` 分别运行 `go test ./...` 验证迁移。
- 回滚策略：如迁移失败，可恢复 `common/credentials` 目录和旧导入；本变更不涉及数据库或配置状态，因此无需数据回滚。

## Open Questions

- 无。
