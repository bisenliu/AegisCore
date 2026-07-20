## 1. 共享密码原语

- [x] 1.1 将 `common/security/password` 替换为 bcrypt 实现，固定 `defaultBcryptCost = 12`，保留 `HashContext` 和 `VerifyContext` 方法签名。
- [x] 1.2 删除 Argon2id 参数、PHC parser、`argon2.IDKey` 派生、queue/gate 门控、`Options` 资源预算和 `ErrPasswordKDFBusy` 相关契约。
- [x] 1.3 将明文密码最大长度收敛到 bcrypt 安全输入上限 72 字节，并保持 `ErrEmptyPassword`、`ErrPasswordTooLong` 和 `ErrInvalidHash` 的稳定匹配语义。
- [x] 1.4 更新 `common/security/password` 单元测试，覆盖 bcrypt hash/verify、错误密码、非法 hash、旧 Argon2id hash 拒绝、空密码、超长密码和 context 预取消。

## 2. user-service 配置和组合

- [x] 2.1 删除 `user-service/internal/config` 中的 `PasswordKDFConfig`、`Auth.PasswordKDF`、`auth.password_kdf.*` 校验和相关测试断言。
- [x] 2.2 删除 `user-service/configs/config.yaml`、`user-service/cmd/serve_test.go`、`user-service/tests/e2e/harness_test.go` 和其他测试 fixture 中的 `auth.password_kdf` 配置块。
- [x] 2.3 更新 `user-service/internal/providers/auth.go` 和 `user-service/cmd/rbac_dependencies.go`，直接构造默认 password service，不再读取 password KDF 或 bcrypt cost 配置。
- [x] 2.4 更新 provider、Fx、bootstrap 和配置加载测试，确保旧 `auth.password_kdf` 配置不再被接受且新默认构造路径通过。

## 3. 认证业务和测试

- [x] 3.1 将 `user-service/internal/features/auth/application/credentials/verifier.go` 的 `dummyPasswordHash` 替换为固定 bcrypt hash，并确保用户不存在时仍执行 dummy verification。
- [x] 3.2 删除认证流程中对 `password.ErrPasswordKDFBusy` 的特殊处理、`MetricsReasonPasswordKDFBusy` 和 `password_kdf_busy` 相关测试或断言。
- [x] 3.3 更新 auth credentials、auth command、auth controller 和 user create 相关测试，确保新建、登录和强制改密生成并验证 bcrypt hash，旧 Argon2id hash 被视为无效凭据。
- [x] 3.4 检查 RBAC `create-super-admin` 和 seed 相关测试，确保创建或显式重置超级管理员密码时写入 bcrypt hash，已有旧 hash 不做兼容处理。

## 4. 规格、文档和生成物检查

- [x] 4.1 同步清理 docs、README、OpenSpec 主规格或 change delta 中残留的 Argon2id、password KDF 预算、`password_kdf_busy` 描述。
- [x] 4.2 确认不修改 `user-service/ent/schema/user.go`、不生成 Atlas migration，使用 `git diff` 检查没有意外 Ent 生成物或 migration 变更。
- [x] 4.3 确认 HTTP 路径和 OpenAPI 请求响应结构未变；如未修改 API 注解，记录无需运行 `make user-service-openapi-generate`。

## 5. 验证和收尾

- [x] 5.1 运行 `go test ./...` in `common/` 并修复失败。
- [x] 5.2 运行 `go test ./...` in `user-service/` 并修复失败。
- [x] 5.3 运行 `make user-service-architecture-lint` 并修复失败。
- [x] 5.4 检查 `git diff`，确认只包含本 change 预期代码、配置、测试、规格和文档变更。
- [x] 5.5 将本次预期变更加到暂存区后运行 `make lint`，通过后保持任务状态可追踪。
- [x] 5.6 在暂存预期变更后运行 `make verify`，通过后完成实现并准备归档。
