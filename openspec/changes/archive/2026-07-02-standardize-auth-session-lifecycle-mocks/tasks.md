## 1. 测试现状梳理

- [x] 1.1 检查 `user-service/internal/features/auth/application/sessions/lifecycle_test.go` 中 `sessionUserTestStore`、`authSessionTestStore` 和 `tokenVersionRecordingInvalidator` 的所有使用点。
- [x] 1.2 确认 `mock_generate.go` 已提供 `UserTokenVersionStore`、`TokenVersionCache`、`RefreshSessionStore` mock，且 `mock_validators_test.go` 已提供 `TokenVersionLocalInvalidator` mock。
- [x] 1.3 将现有测试按 token version 回源、refresh session 创建/旋转、当前会话退出、全量撤销和本地 cache 失效路径分组，确定每组需要的 expectation。

## 2. gomock 改造

- [x] 2.1 为 lifecycle 测试引入 `gomock.Controller` 和最小测试 helper，统一装配生成 mock 与 session lifecycle use case。
- [x] 2.2 将 token version 回源相关测试改为使用 `UserTokenVersionStore` 和 `TokenVersionCache` mock，并用 matcher 或 `DoAndReturn` 表达 Redis miss、PostgreSQL 回源和 Redis 回填路径。
- [x] 2.3 将 refresh session 创建和旋转相关测试改为使用 `RefreshSessionStore` mock，并明确表达 session 参数、rotation 成功和失败分支 expectation。
- [x] 2.4 将当前会话退出与全量撤销相关测试改为使用生成 store mock 表达 revoke、delete-all、token version 提升和投影刷新失败信号。
- [x] 2.5 将本地 token version cache 失效相关测试改为使用 `TokenVersionLocalInvalidator` mock，并在安全敏感路径上使用顺序 expectation。
- [x] 2.6 删除 `sessionUserTestStore`、`authSessionTestStore` 和 `tokenVersionRecordingInvalidator` 定义，不保留兼容路径。

## 3. 验证与收尾

- [x] 3.1 执行 `make user-service-generate`，确认 mockgen 产物无 drift，并检查 `git diff --exit-code -- user-service/internal/features/auth/application/sessions/mock_generate.go user-service/internal/features/auth/application/sessions/mock_validators_test.go`。
- [x] 3.2 执行 `cd user-service && go test ./internal/features/auth/application/sessions`，确认 sessions 包测试通过。
- [x] 3.3 执行 `make user-service-architecture-lint`，确认架构边界检查通过。
- [x] 3.4 确认 `lifecycle_test.go` 中不再存在 `sessionUserTestStore`、`authSessionTestStore` 或 `tokenVersionRecordingInvalidator`。
- [x] 3.5 暂存本次预期代码、测试和 OpenSpec 产物变更后执行 `make lint`，通过后再执行 `make verify`，任一失败时修复后重跑。
