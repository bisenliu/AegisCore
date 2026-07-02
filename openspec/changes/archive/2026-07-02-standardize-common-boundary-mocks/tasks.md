## 1. 工具入口与生成命令

- [x] 1.1 在 `common/go.mod` 中声明 `go.uber.org/mock/mockgen` tool 依赖，并同步 `common/go.sum`。
- [x] 1.2 在 `common/security/casbin` 增加 package-local `mock_generate.go`，为 `Enforcer` 生成测试 mock。
- [x] 1.3 在 `common/http/middleware` 增加 package-local `mock_generate.go`，为 `CasbinAuthorizer` 和 `auth.TokenVersionValidator` 生成测试 mock。
- [x] 1.4 在 `common/Makefile` 增加 `generate` 目标，并让 `verify` 覆盖 lint、generate、test 和生成物 drift 检查。
- [x] 1.5 在根 `Makefile` 增加 `common-generate` 目标，并让根 `make verify` 执行 common 生成命令。

## 2. common 测试迁移

- [x] 2.1 执行 `make common-generate` 或 `make -C common generate`，提交 `common/security/casbin` 与 `common/http/middleware` 的 package-local mock 生成物。
- [x] 2.2 将 `common/security/casbin/authorizer_test.go` 的 `recordingEnforcer` 迁移为 generated mock expectation，并删除手写 double。
- [x] 2.3 将 `common/http/middleware/casbin_test.go` 的 `recordingCasbinAuthorizer` 迁移为 generated mock expectation，并删除手写 double。
- [x] 2.4 将 `common/http/middleware/auth_test.go` 中用于 token version 调用断言的 `recordingTokenVersionValidator` 迁移为 generated mock expectation，并删除手写 double。
- [x] 2.5 确认未创建 `common/mocks`、全局 `mocks/`、`testmocks/` 或中央 mock 仓库，且未为测试新增业务无关生产接口、adapter、分支或 `NewXForTest`。

## 3. 验证与收口

- [x] 3.1 运行 `openspec validate standardize-common-boundary-mocks`。
- [x] 3.2 运行 `make user-service-architecture-lint`，确认 OpenSpec Markdown 和架构边界检查通过。
- [x] 3.3 运行 `make common-test`。
- [x] 3.4 运行 `make common-verify`，确认 common 生成物无 drift。
- [x] 3.5 清理临时文件并暂存本次预期代码、生成物、Makefile 和 OpenSpec 变更。
- [x] 3.6 在暂存预期变更后运行 `make lint`。
- [x] 3.7 在暂存预期变更后运行 `make verify`，确认最终 `git diff --exit-code` 无未暂存 drift。
