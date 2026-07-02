## Context

`user-service/internal/features/auth/transport/http/controller_test.go` 当前用手写 `stubAuthUseCases` 同时模拟登录、刷新、改密和退出 use case。这个 stub 通过测试私有字段记录 `LoginCommand`、`RefreshTokenCommand`、`ChangePasswordCommand` 和 `authctx.ClientContext`，可以覆盖现有断言，但它不是仓库内 user/permission HTTP controller 已采用的 `go.uber.org/mock/gomock` 生成 mock 风格。

user/permission HTTP transport 已在同目录维护 `mock_generate.go`，通过 `go tool mockgen` 将 feature application 层接口生成到 transport 测试包内。auth application 层已有 `LoginUseCase`、`RefreshTokenUseCase`、`ChangePasswordUseCase`、`LogoutCurrentSessionUseCase`、`LogoutAllSessionsUseCase` 五个最小接口，适合直接生成 feature-local mock，不需要新增生产接口或跨 feature mock 包。

## Goals / Non-Goals

**Goals:**

- 让 auth HTTP controller 测试使用 `gomock` expectation 表达 use case 调用契约。
- 新增 auth HTTP transport 本地 `mock_generate.go`，并让 `make user-service-generate` 可重复生成 mock。
- 删除 `stubAuthUseCases` 及其专用状态字段，避免测试同时维护手写 stub 和生成 mock 两套表达方式。
- 保持命令归一化、client context 注入、认证错误映射、KDF busy 映射和 token invalid 映射的现有覆盖。

**Non-Goals:**

- 不修改认证 HTTP 路由、DTO、错误码、OpenAPI 注解或生产 controller 行为。
- 不修改 auth application use case 实现或接口签名。
- 不新增 `NewXForTest`、测试专用生产接口、全局 `mocks/` 目录或跨 feature mock 包。
- 不调整数据库 schema、migration、部署清单或观测资产。

## Decisions

- 在 `auth/transport/http` 包内新增 `mock_generate.go`，使用和 user/permission HTTP transport 一致的 `//go:generate go tool mockgen` 入口。
  备选方案是在 application/command 包集中生成 mock，但 controller 测试消费侧在 transport 包，feature-local transport mock 更接近现有 user/permission 模式，也避免应用层测试 mock 与 HTTP 边界测试 mock 混用。

- 将五个 use case 接口生成到 auth HTTP transport 测试包内。
  备选方案是生成一个组合接口 mock，但生产代码没有这个组合接口；新增组合接口只为测试存在，会扩大生产 API 面并违反“不新增测试专用生产接口”的边界。

- 在 controller 测试中用 `gomock.NewController(t)` 和各 use case mock 组装 `AuthControllerParams`。
  备选方案是保留 `stubAuthUseCases` 作为 helper 包装生成 mock，但这会继续保留旧入口，无法达成“只通过生成 mock 表达契约”的目标。

- 对参数归一化和 client context 注入使用 `gomock` matcher 或 `DoAndReturn` 断言。
  备选方案是只使用 `gomock.Any()` 并检查响应，但这会降低测试对 HTTP input preparer 和 context 注入行为的覆盖。

## Risks / Trade-offs

- 生成的 mock 文件会增加测试代码体积 -> 通过 `make user-service-generate` 和 drift 校验保证文件可再生成，不手写生成物。
- `gomock` expectation 过细可能让测试对无关实现顺序敏感 -> 只断言 use case 方法、归一化命令、client context 和错误映射，不断言 controller 内部步骤顺序。
- 多个 use case mock 会让测试 setup 比单个 stub 更显式但更冗长 -> 使用测试 helper 统一创建 controller，仍由每个测试设置自己的 expectation。
- 若新增 auth use case 后忘记更新 mockgen 入口，生成物可能 drift -> 通过 `make user-service-generate` 和 `make verify` 暴露。

## Migration Plan

1. 新增 `user-service/internal/features/auth/transport/http/mock_generate.go`，声明五个 auth command use case 的 `mockgen` 入口。
2. 执行 `make user-service-generate` 生成 auth HTTP transport 测试包内 mock 文件。
3. 改造 `controller_test.go`，用生成 mock 替换 `stubAuthUseCases`，保留现有行为断言并补足 use case expectation。
4. 删除 `stubAuthUseCases` 和只服务于该 stub 的旧辅助字段。
5. 执行 `cd user-service && go test ./internal/features/auth/transport/http`、`make user-service-generate`、`make user-service-architecture-lint` 验证。

回滚方式：恢复 `controller_test.go` 到旧 stub 测试、删除 auth HTTP transport `mock_generate.go` 和生成 mock 文件；该变更不涉及生产行为、数据库或部署资产，回滚不需要数据迁移。

## Open Questions

- 无未决问题。现有 user/permission HTTP transport mockgen 模式可直接复用。
