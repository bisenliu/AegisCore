# Tasks

## Preparation

- [x] 阅读 `AGENTS.md` 和 `docs/ARCHITECTURE.md`，确认本仓库不新增 `openspec/` 或 `docs/opsx/`。
- [x] 确认 change 目录使用 `docs/changes/add-auth-failure-client-context-logging/`。
- [x] 梳理现有 request logging、auth middleware 和 auth login failure 日志位置。
- [x] 确认本变更不修改认证业务语义、HTTP API、数据库 schema、Redis key 或配置。

## Common HTTP Middleware

- [x] 在 `common/http/middleware` 新增或复用 helper，集中生成 request 和 auth failure 日志字段。
- [x] 将 request log 延迟字段从 `latency` 标准化为 `latency_ms`。
- [x] 确认 request log 继续包含 `method`、`path`、`status`、`client_ip`、`user_id` 和 trace-id。
- [x] 更新 `AuthWithTokenVersionValidator` 的 missing authorization header 日志，增加 `client_ip` 和 `user_agent`。
- [x] 更新 invalid authorization header format、empty bearer token 日志，增加 `client_ip` 和 `user_agent`。
- [x] 更新 token validation failed 日志，增加 `client_ip` 和 `user_agent`，且不记录 token 原文。
- [x] 更新 token version mismatch 日志，保留版本字段并增加 `client_ip` 和 `user_agent`。
- [x] 更新 token version validation failed 日志，增加 `client_ip` 和 `user_agent`。
- [x] 确认认证失败预期拒绝仍为 `Info` 或 `Warn`，系统配置或依赖失败仍为 `Error`。

## Auth Login Failure Context

- [x] 在 auth application command 层增加 transport-neutral client context helper。
- [x] 在 auth HTTP controller 登录入口把 `client_ip` 和 `user_agent` 写入 request context 后再调用 `Login`。
- [x] 在登录失败日志中追加 context 中的 `client_ip` 和 `user_agent`。
- [x] 覆盖 `login user not found`、`login password mismatch` 和 `login user status rejected`。
- [x] 确认 application 层不导入 Gin、HTTP request、HTTP DTO、HTTP response envelope 或 binder。
- [x] 确认不记录 password、token、Authorization header、Cookie 或原始请求体。

## Documentation

- [x] 更新 `docs/ARCHITECTURE.md` 日志规则，使 trace-id/context key 与日志字段命名说明一致。
- [x] 更新 `docs/DEVELOPMENT.md` 日志说明，补充 access log 标准字段和认证失败安全事件字段。
- [x] 如根规则需要同步，更新 `AGENTS.md` 中日志字段标准说明。
- [x] 确认文档不重新引入 OpenSpec/OPSX 流程或目录。

## Tests

- [x] 更新 `common/http/middleware/auth_test.go`，断言认证失败日志包含 `client_ip` 和 `user_agent`。
- [x] 更新 token version mismatch 测试，断言版本字段与客户端上下文同时存在。
- [x] 更新 `common/http/middleware/trace_id_test.go`，将 request log 延迟字段断言改为 `latency_ms`。
- [x] 更新 `user-service/internal/providers/routes_test.go` 中 request log 字段断言。
- [x] 为 auth 登录失败客户端上下文增加单元测试或窄集成测试。
- [x] 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 运行 common middleware 测试：

```bash
cd common && go test ./http/middleware
```

- [x] 运行 auth 和 provider 相关测试：

```bash
cd user-service && go test ./internal/features/auth/... ./internal/providers
```

## Verification

- [x] 扫描确认没有敏感字段进入日志：

```bash
rg -n 'password|access_token|refresh_token|authorization|cookie' common user-service --glob '*.go'
```

- [x] 扫描确认字段命名已更新：

```bash
rg -n 'latency|latency_ms|client_ip|user_agent' common user-service --glob '*.go'
```

- [x] 检查文档和代码变更范围：

```bash
git diff --stat
git diff -- docs/changes/add-auth-failure-client-context-logging docs/ARCHITECTURE.md docs/DEVELOPMENT.md AGENTS.md common user-service
```

## Guardrails

- [x] 不新增 `openspec/` 或 `docs/opsx/`。
- [x] 不修改 HTTP response envelope、错误码、响应 message 或 status code。
- [x] 不修改 Ent schema、Atlas migration、Redis key schema 或配置 key。
- [x] 不让 application 层依赖 Gin、HTTP binder、HTTP response helper、Ent 或 Redis client。
- [x] 不记录明文密码、token、Authorization header、Cookie 或原始请求体。
- [x] 不把预期认证拒绝记录为 `Error`。
