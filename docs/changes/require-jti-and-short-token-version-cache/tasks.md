# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只收紧 JWT `jti` 与 token version cache 默认 TTL。
- [x] 检查 `common/security/auth/jwt.go` 当前 `Claims`、`JWTService.sign`、`JWTService.parse`、三类 parse wrapper 的行为。
- [x] 检查 `common/security/auth/jwt_test.go` 当前签发和解析测试。
- [x] 检查 `user-service/configs/config.yaml`、`common/runtime/config/loader_test.go`、`user-service/internal/bootstrap/http_test.go` 中默认 TTL 断言。
- [x] 确认本次不实现 Redis token denylist、单 token 撤销 API、Redis key schema 修改、Ent schema 修改或 migration。

## JWT jti

- [x] 在 `common/security/auth/jwt.go` 新增缺失 token ID 错误，例如 `ErrMissingTokenID`。
- [x] 如采用合法 UUID 强校验，新增畸形 token ID 错误，例如 `ErrInvalidTokenID`。
- [x] 在 `JWTService.sign` 的 `jwtv5.RegisteredClaims` 中设置 UUID v7 `jti`。
- [x] 在 `JWTService.parse` 中校验 `claims.ID` 非空。
- [x] 如采用合法 UUID 强校验，在 `JWTService.parse` 中使用 `uuid.Parse(claims.ID)` 校验 `jti`；解析阶段不限制 UUID version。
- [x] 保持 `Claims` 不新增自定义 `JTI` 字段。
- [x] 保持 access、refresh、password_change token 共用同一 `sign` 和 `parse` 约束。

## Token Version Cache TTL

- [x] 将 `user-service/configs/config.yaml` 中 `auth.token_version_cache_ttl` 从 `5m` 修改为 `30s`。
- [x] 更新配置注释，说明该 TTL 是 Redis token version 投影的最长缓存窗口，缓存 miss 仍回源 PostgreSQL。
- [x] 更新 `common/runtime/config/loader_test.go` 中依赖默认或显式样例 TTL 的期望值。
- [x] 更新 `user-service/internal/bootstrap/http_test.go` 中默认 auth TTL 断言。
- [x] 使用 `rg -n "token_version_cache_ttl: 5m|5\\*time\\.Minute|want \\(15m,168h,5m\\)"` 检查是否还有需要同步的测试或文档。

## Tests

- [x] 更新 `TestJWTServiceSignTokens`，断言新签发 access token 的 `claims.ID` 非空且是合法 UUID v7。
- [x] 更新或新增测试，断言新签发 refresh token 的 `claims.ID` 非空且是合法 UUID v7。
- [x] 更新或新增测试，断言新签发 password_change token 的 `claims.ID` 非空且是合法 UUID v7。
- [x] 新增缺少 `jti` 的旧 access token 测试，期望 `ErrMissingTokenID`。
- [x] 新增缺少 `jti` 的旧 refresh token 测试，期望 `ErrMissingTokenID`。
- [x] 新增缺少 `jti` 的旧 password_change token 测试，期望 `ErrMissingTokenID`。
- [x] 如实现 `ErrInvalidTokenID`，新增畸形 `jti` 测试，期望 `ErrInvalidTokenID`。
- [x] 更新配置加载测试，断言默认 TTL 为 `30s`。

## Guardrails

- [x] 不新增 `openspec/` 或 `docs/opsx/` 工件。
- [x] 不新增横向 `internal/shared`、`internal/service`、`internal/repository` 或其他违反 feature-first 结构的目录。
- [x] 不新增 Redis denylist、blacklist、outbox、eventbus、MQ 或后台 worker。
- [x] 不改变 `/auth/logout` 与 `/auth/logout-all` 的语义。
- [x] 不改变 HTTP response envelope、错误码契约或 Swagger model，除非实现阶段明确需要暴露新 API 行为。
- [x] 不修改 Ent generated code、Ent schema、Atlas migration 或 PostgreSQL 表结构。
- [x] 保持源码注释为中文，日志消息为英文。

## Verification

- [x] 运行 common JWT 单包测试：

```bash
cd common
go test ./security/auth
```

- [x] 运行 common 配置测试：

```bash
cd common
go test ./runtime/config
```

- [x] 运行 user-service bootstrap 测试：

```bash
cd user-service
go test ./internal/bootstrap
```

- [x] 如时间允许，运行变更范围测试：

```bash
make test-common
make test-user-service
```

- [x] 检查新 token payload 中包含标准 `jti`，且没有自定义 `JTI` 字段。
- [x] 检查缺少 `jti` 的旧 token 被拒绝，确认本次不保留旧 token 兼容窗口。

## Release Notes

- [x] 在发布说明中明确：本变更会使缺少 `jti` 的旧 access token、refresh token 和 password_change token 失效，用户需要重新登录。
- [x] 说明默认 `token_version_cache_ttl` 从 `5m` 缩短为 `30s`。
- [x] 本次未执行 Redis 清理；发布时如运维执行清理，需要记录清理范围和回滚注意事项。
