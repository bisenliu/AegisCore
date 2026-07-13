## 1. 配置与资源边界

- [x] 1.1 新增 user-service 私有配置包，定义服务根配置、`AuthConfig`、`JWTConfig`、`PasswordKDFConfig`、`EntConfig` 和 `Validate`，并复用 common loader 的通用读取能力。
- [x] 1.2 将 `common/runtime/config` 收敛为通用 runtime 配置，移除 `Auth`、`Ent`、认证策略校验和 user-service 语义测试。
- [x] 1.3 调整 user-service bootstrap、CLI、Fx provider、测试 fixture 和 e2e harness，使它们注入并消费 user-service 私有配置类型。
- [x] 1.4 新增 user-service 私有资源名常量包，迁移 `NameUserDB` 和服务级 Redis/cache 资源名引用，并从 `common/runtime/resources` 删除用户服务资源名。
- [x] 1.5 更新 datastore、Ent provider、RBAC CLI 和相关测试，确保通用 datastore 只接收调用方传入的资源名字符串。

## 2. JWT verifier 与 issuer 拆分

- [x] 2.1 将 `common/security/auth` 改造为 verifier-only primitive，保留 HMAC 验签、issuer/audience 校验、Bearer helper 和通用错误，删除签发 API、user-service claims 和 refresh/password-change subject。
- [x] 2.2 在 user-service auth token 包实现私有 issuer、claims、subject、TTL fallback、access/refresh/password-change 签发和解析校验。
- [x] 2.3 调整登录、refresh、强制改密、session lifecycle 和 token version 校验链路，使业务逻辑只依赖 user-service 私有 token issuer/verifier 接口。
- [x] 2.4 迁移 JWT 单元测试：common 只覆盖通用 verifier 行为，user-service 覆盖三类 token 签发、subject 拒绝、claims 字段、TTL fallback 和 token 缺少 `jti`。

## 3. HTTP middleware 最小权限接入

- [x] 3.1 调整共享 HTTP auth middleware constructor，使其依赖访问令牌 verifier 最小接口和可选 token version validator，而不是具备签发能力的 concrete JWT service。
- [x] 3.2 在 user-service provider 边界实现 access token verifier adapter，完成 user-service claims 解析、subject 校验、`user_id`/`session_id`/`token_version` 校验和认证上下文映射。
- [x] 3.3 更新 router/provider/auth middleware 测试，覆盖无效 token、subject 错配、token version mismatch、middleware 不持有签发能力和受保护路由行为不变。

## 4. 规格、文档与架构检查

- [x] 4.1 更新 `docs/opsx/CAPABILITY_MAP.md` 和相关架构文档，使 `shared-platform-primitives` 不再声明 user-service JWT/资源名职责，`auth-session-management` 拥有 issuer 和服务私有认证配置。
- [x] 4.2 运行 `make user-service-architecture-lint`，修复 common、user-service、internal/shared 和 feature 分层边界违规。
- [x] 4.3 确认 HTTP API、OpenAPI 和数据库 schema 未变化；若生成脚本产生 diff，说明原因并只保留与本 change 必需相关的变更。

## 5. 验证与交付

- [x] 5.1 运行相关包测试：`go test ./common/runtime/config ./common/runtime/datastore ./common/security/auth ./common/http/middleware ./user-service/internal/providers ./user-service/internal/features/auth/... ./user-service/cmd`。
- [x] 5.2 运行 `make test`，修复本 change 引入的失败。
- [x] 5.3 将本次预期代码、规格和文档变更加到暂存区，避免最终 verify 的 git diff 检查被预期变更阻塞。
- [x] 5.4 运行 `make lint`，修复 lint 失败。
- [x] 5.5 运行 `make verify`，确认完整验证通过并检查生成物无非预期 drift。
- [x] 5.6 更新本 `tasks.md` checkbox 状态，仅在对应实现和验证真实完成后标记 `- [x]`。
