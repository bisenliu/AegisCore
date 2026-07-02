## 1. 迁移准备

- [x] 1.1 梳理 `user-service/internal/features/auth/application/command/service_test.go` 中 `authCredentialTestStore`、`authSessionTestStore`、`recordingAuthMetrics`、`refreshRotationTokenIssuer` 的所有使用点，并按登录、刷新、改密、退出当前会话、退出全部会话分组。
- [x] 1.2 确认 `mock_generate.go` 已覆盖 `UserCredentialStore`、`UserTokenVersionStore`、`TokenVersionCache`、`RefreshSessionStore`、`Metrics`、`Verifier`、`Issuer`、`Lifecycle`，不新增跨包共享 mock 仓库。

## 2. 测试迁移

- [x] 2.1 将登录相关测试改为使用生成 mock，并通过 expectation 表达 credential store、password verifier、token issuer、refresh session store 和 metrics 调用。
- [x] 2.2 将 refresh token 相关测试改为使用生成 mock，并通过 expectation、`gomock.InOrder`、matcher 或 `DoAndReturn` 表达 token 解析、session 查询、token version 校验、rotation 和失败指标。
- [x] 2.3 将修改密码相关测试改为使用生成 mock，并明确断言 credential 更新、token version 递增、本地缓存失效、Redis 投影刷新和 metrics 调用。
- [x] 2.4 将退出当前会话相关测试改为使用生成 mock，并明确断言当前 refresh session 删除、不会递增 token version 和 metrics 调用。
- [x] 2.5 将退出全部会话相关测试改为使用生成 mock，并明确断言 token version 递增、本地缓存失效、Redis 投影刷新、全部 session 删除、purge 提交失败和 metrics 调用。

## 3. 清理与生成物

- [x] 3.1 删除 `service_test.go` 中不再使用的 `authCredentialTestStore`、`authSessionTestStore`、`recordingAuthMetrics`、`refreshRotationTokenIssuer` 类型及其方法。
- [x] 3.2 保留只负责构造输入、领域对象或真实轻量纯函数依赖的 helper，确认不存在实现外部 collaborator port 的新手写 double。
- [x] 3.3 执行 `make user-service-generate`，检查 `mock_*.go` 无 mockgen drift。

## 4. 验证

- [x] 4.1 执行 `cd user-service && go test ./internal/features/auth/application/command`。
- [x] 4.2 执行 `make user-service-architecture-lint`。
- [x] 4.3 执行 `openspec validate standardize-auth-command-collaborator-mocks`。
- [x] 4.4 暂存本次预期代码、测试和 OpenSpec 变更后执行 `make lint`。
- [x] 4.5 在本次预期变更已暂存的前提下执行 `make verify`，确认没有生成物 drift 或未纳入暂存区的意外变更。
