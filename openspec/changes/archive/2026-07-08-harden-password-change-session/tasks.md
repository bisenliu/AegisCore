## 1. 配置与 token TTL

- [x] 1.1 在 `common/runtime/config` 中新增 `auth.jwt.password_change_token_ttl` 字段、默认值处理和配置校验测试。
- [x] 1.2 更新 `user-service/configs/config.yaml`，将 `password_change_token_ttl` 设置为 `5m` 并补充配置注释。
- [x] 1.3 调整 `user-service/internal/features/auth/application/tokens`，使 `IssuePasswordChangeToken` 使用独立 TTL，并补充不再复用 `access_token_ttl` 的单元测试。
- [x] 1.4 确认 JWT `jti` 解析结果可供 auth application 使用；如现有 claims 已包含 `RegisteredClaims.ID`，仅补充测试覆盖 password-change `jti` 传递。

## 2. Password-change session 存储

- [x] 2.1 在 auth application port 中定义 `PasswordChangeSessionStore` 和 password-change session 领域模型，字段包含 `user_id`、`session_id`、`token_id`、`token_version` 和 `expires_at`。
- [x] 2.2 在 `user-service/internal/features/auth/infrastructure/redis` 增加 password-change session key schema，不复用 refresh session key、索引或上限裁剪逻辑。
- [x] 2.3 实现创建 password-change session 的 Redis adapter，TTL 使用 `password_change_token_ttl`，非正数 TTL 不创建永久 key。
- [x] 2.4 实现原子消费 Lua 脚本，单次校验 `user_id`、`session_id`、`token_id` 和 `token_version` 后删除 key。
- [x] 2.5 实现撤销 password-change session 能力，用于服务端删除未消费的一次性会话。
- [x] 2.6 补充 Redis adapter 测试，覆盖创建成功、过期、撤销、claims 不一致、重复消费和并发消费只有一个成功。

## 3. Auth use case 行为收紧

- [x] 3.1 调整 auth Fx provider 和 command dependencies，为登录与改密 use case 注入 password-change session store 的最小依赖。
- [x] 3.2 调整强制改密登录分支，签发 token 后创建 password-change session；session 创建失败时不返回 token。
- [x] 3.3 调整 `ChangePassword`，在更新凭据前解析 password-change claims 并原子消费 password-change session，所有消费失败统一映射为 `ErrTokenInvalid`。
- [x] 3.4 将 `ValidatePasswordChangeClaims` 的职责替换为 password-change session 消费和必要的 token version 校验，移除仅依赖当前 `token_version` 的改密授权路径。
- [x] 3.5 更新 gomock 生成物和 command 单元测试，覆盖登录创建一次性会话、创建失败不返回 token、复用/过期/撤销/claims 不一致/并发消费失败。

## 4. 凭据条件更新与撤销错误策略

- [x] 4.1 在 auth credential port 和 PostgreSQL adapter 中新增按旧 `token_version` 与强制改密状态条件更新凭据的方法。
- [x] 4.2 更新 `ChangePassword` 使用条件更新方法，状态不匹配、版本不匹配或用户不存在时统一返回无效凭据或既有 not found 语义。
- [x] 4.3 调整 `RevokeUserSessionsAtVersion` 调用方策略，使改密后本地缓存失效、Redis token version 投影刷新或 refresh session 删除失败时返回安全撤销未完成错误。
- [x] 4.4 新增 `ErrSessionRevocationIncomplete` 或等价领域错误，并在 HTTP mapper 中映射为 `503 Service Unavailable`，响应不得泄露内部错误。
- [x] 4.5 移除或改写当前“投影失败仍成功”的测试，新增撤销投影失败返回 503 和不返回 `Changed: true` 的 use case/HTTP 测试。
- [x] 4.6 补充 PostgreSQL adapter 测试，覆盖状态匹配更新、状态不匹配拒绝、旧 `token_version` 不匹配拒绝和并发条件更新只成功一次。

## 5. Metrics、告警与 OpenAPI

- [x] 5.1 扩展 auth feature-local `Metrics` interface 和 no-op 生成物，增加 password-change session 消费失败、重复消费拒绝、撤销投影失败和补偿失败指标。
- [x] 5.2 实现 Prometheus 指标记录，标签只使用固定低基数枚举，不包含用户 ID、session ID、jti、token、Redis key 或原始错误。
- [x] 5.3 更新 `deployments/observability` Prometheus alert rules，覆盖强制改密撤销投影失败和补偿失败。
- [x] 5.4 更新 Grafana dashboard 或 metrics load 验证脚本，覆盖新增强制改密安全指标 presence 或 PromQL 查询。
- [x] 5.5 更新认证 HTTP 注解并运行 `make user-service-openapi-generate`，确保登录 `expires_in` 和改密 503 错误说明同步到 OpenAPI 生成物。

## 6. 规格、验证与收尾

- [x] 6.1 运行 auth 相关包测试：`go test ./user-service/internal/features/auth/...`。
- [x] 6.2 运行 common 配置和 JWT 相关测试：`go test ./common/runtime/config ./common/security/auth`。
- [x] 6.3 运行观测资产或 dashboard 校验：`make compose-dashboard-check` 或对应新增/更新脚本。
- [x] 6.4 运行架构和 OpenSpec 相关校验：`make user-service-architecture-lint`。
- [x] 6.5 检查生成物 drift：`git diff --exit-code -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml deployments/observability deployments/compose/grafana`。
- [x] 6.6 将本次预期代码、规格和生成物暂存后运行 `make lint`。
- [x] 6.7 暂存本次预期变更后运行 `make verify`，确保最终验证不被未暂存预期 diff 阻塞。
