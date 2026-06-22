## Why

`token_version_validator_test.go` 中的 `testTokenVersionValidator` wrapper 只额外持有一个未被测试断言使用的 cache 字段，使测试 helper 的返回类型比实际需要更复杂。

现在清理该 wrapper，可以让测试直接围绕 `*TokenVersionValidator` 表达意图，并保持 token version 校验、缓存、失效和回源语义不变。

## What Changes

- 删除 `user-service/internal/features/auth/application/validators/token_version_validator_test.go` 中的 `testTokenVersionValidator` 测试 wrapper 类型。
- 调整 `newTestTokenVersionValidator`，使其直接返回 `*TokenVersionValidator`。
- 将 local cache 仅作为 helper 局部变量保留，用于构造 validator 并注册 `t.Cleanup(cache.Close)`。
- 保持现有测试用例、断言、覆盖面和生产代码行为不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 整理 token version validator 测试辅助结构，保持 token version 校验链路、缓存、失效和回源需求语义不变。

## Impact

- 影响代码范围：`user-service/internal/features/auth/application/validators/token_version_validator_test.go`。
- 不影响 API、数据库 schema、OpenAPI、部署、观测、安全契约或共享契约。
- 验证范围：运行 auth validators 相关 Go 测试，确认现有 token version validator 测试语义保持不变。
