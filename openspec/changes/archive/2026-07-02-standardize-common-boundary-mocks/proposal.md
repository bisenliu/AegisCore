## Why

`common` 模块中的边界测试仍保留多处手写 recording double，用于记录 Casbin 授权、HTTP 授权和 token version 校验调用。此类 double 适合用 expectation mock 表达，继续手写会增加重复维护成本，也会让 `common` 与 `user-service` 已建立的生成化 mock 规范产生漂移。

本 change 聚焦 `common` 内可复现 mock 生成与包内测试替换，不改变生产接口语义、HTTP middleware 行为、JWT/token version 业务规则或状态型测试 harness。

## What Changes

- **BREAKING** 删除纳入范围的 `common` 手写 recording double，改用 `go.uber.org/mock` 生成的 package-local 测试 mock。
- 为 `common/security/casbin.Enforcer`、`common/http/middleware.CasbinAuthorizer`、`common/security/auth.TokenVersionValidator` 等边界 interface 增加可复现 mockgen 入口。
- 在 `common` 模块声明 `go.uber.org/mock/mockgen` 工具依赖或等价可复现工具入口，生成流程不得依赖全局 `mockgen` 二进制。
- 在 `common/Makefile` 和根 `Makefile` 中补齐 common 生成与 drift 校验入口，确保 `make common-verify` 和完整 `make verify` 覆盖 common mockgen drift。
- 不创建 `common/mocks`、全局 `mocks/` 或中央 mock 仓库；mock 生成物必须留在对应 package 或 package-local 测试边界。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 增加 `common` 边界 interface 的 package-local mock 生成规范，要求适合 expectation 表达的手写 recording double 迁移为生成 mock。
- `delivery-operations`: 增加 `common` 模块 mockgen 工具入口、生成命令和 drift 校验要求，确保 common 生成物可复现并纳入完整验证。

## Impact

- 影响 Go 测试代码：`common/security/casbin/authorizer_test.go`、`common/http/middleware/casbin_test.go`、`common/http/middleware/auth_test.go` 中适合 expectation 表达的手写 recording double 将被替换。
- 影响 Go 生成物：新增 package-local mock 文件和对应 `go:generate` 入口。
- 影响依赖和命令：`common` 模块需要显式声明 mockgen 工具依赖或工具入口；`common/Makefile` 与根 `Makefile` 需要补齐 common 生成和 verify 目标。
- 不影响 HTTP API、数据库 schema、OpenAPI 输出、部署资产、JWT 解析语义、token version 校验语义、Casbin 授权三元组语义或 scheduler 状态型测试 harness。
