## Why

`auth/application/command` 的 use case 测试仍保留多套手写 collaborator double，导致登录、刷新、改密和退出流程的依赖调用、失败路径与指标记录断言分散在状态检查和兼容替身内部。该包已经拥有 `mockgen` 生成物，本次变更将测试协作者统一迁移到生成 mock，使测试意图更直接，并降低后续 auth session 行为演进时维护两套测试替身的成本。

## What Changes

- 将 `user-service/internal/features/auth/application/command` 包内 credential、session、token version、token issuer/verifier、metrics 和 lifecycle 等外部协作者改为使用已有 gomock 生成物。
- 移除 `service_test.go` 中 `authCredentialTestStore`、`authSessionTestStore`、`recordingAuthMetrics`、`refreshRotationTokenIssuer` 等旧手写 collaborator double。
- 用 `gomock` expectation、`gomock.InOrder`、自定义 matcher 或 `DoAndReturn` 表达登录、刷新、强制改密、退出当前会话和退出全部会话的依赖调用、失败路径与指标记录。
- 保留纯构造类测试 helper，不改变生产代码、JWT 签发实现、密码服务实现、Redis store 或配置语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 明确认证 command use case 测试的外部协作者应通过生成 mock 表达依赖契约，覆盖登录、刷新、改密、退出当前会话和退出全部会话的调用与失败路径验证，不改变运行时认证会话行为。

## Impact

- 受影响代码限定在 `user-service/internal/features/auth/application/command` 包测试和该包已有 `mock_generate.go` 覆盖的生成 mock。
- 不影响 HTTP API、OpenAPI、数据库 schema、Atlas migration、Redis key 语义、JWT claims、配置项或部署资产。
- 需要验证 `make user-service-generate` 无 mockgen drift、`cd user-service && go test ./internal/features/auth/application/command` 通过，以及 `make user-service-architecture-lint` 通过。
