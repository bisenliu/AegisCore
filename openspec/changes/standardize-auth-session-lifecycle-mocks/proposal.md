## Why

`auth/application/sessions` 的 session lifecycle 测试仍依赖手写 store 与 invalidator，导致 token version 回源、refresh session 旋转、全量撤销和本地缓存失效的交互验证散落在测试状态字段中，不够显式也容易与已生成的 gomock 契约产生漂移。

本变更通过统一使用现有生成 mock，让测试直接表达端口调用、参数匹配、调用顺序和错误分支，降低后续维护 auth session lifecycle 行为时的回归风险。

## What Changes

- 移除 `lifecycle_test.go` 中的 `sessionUserTestStore`、`authSessionTestStore` 和 `tokenVersionRecordingInvalidator` 手写测试替身。
- 使用已有 `mock_generate.go` 生成的 `UserTokenVersionStore`、`TokenVersionCache`、`RefreshSessionStore` mock 覆盖 session lifecycle store 依赖。
- 使用已有 `mock_validators_test.go` 生成的 `TokenVersionLocalInvalidator` mock 覆盖本地 token version cache 失效依赖。
- 将旧测试中的记录字段、回调和状态断言迁移为 gomock matcher、`DoAndReturn` 或顺序 expectation。
- 保持 session lifecycle 生产代码、外部接口、Redis session store 测试、auth command 测试和 token version validator 测试不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 不改变稳定业务需求；本 change 仅加强该能力下 session lifecycle 测试对既有 token version、refresh session 和撤销语义的显式验证。

## Impact

- 影响代码范围限定在 `user-service/internal/features/auth/application/sessions` 包内测试。
- 不影响生产代码、HTTP API、OpenAPI、数据库 schema、Redis key schema、部署资产或共享契约。
- 依赖现有 mockgen 产物，不新增测试专用生产构造函数或冗余 adapter。
- 验证命令包括 `make user-service-generate`、`cd user-service && go test ./internal/features/auth/application/sessions` 和 `make user-service-architecture-lint`。
