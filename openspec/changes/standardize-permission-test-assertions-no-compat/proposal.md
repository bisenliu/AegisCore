## Why

permission feature 的历史 Go 测试中仍存在较多 `t.Fatal`、`t.Errorf` 和手写失败判断，和 `docs/TESTING.md` 中优先使用 `testify/require` 语义化断言的规范不一致，导致失败信息、级联失败控制和维护方式不统一。

本次变更在不修改 permission 生产行为和 RBAC 授权语义的前提下，统一 permission 测试断言风格，并明确不为旧字段、旧 scanner 输出、旧 watcher 签名或旧授权白名单保留兼容断言。

## What Changes

- 将 `user-service/internal/features/permission/**/*_test.go` 中的历史手写断言迁移为 `testify/require` 语义化断言。
- 对 route diff 或多字段响应中互相独立的字段失败收集，允许按 `docs/TESTING.md` 使用 `testify/assert`。
- 保持已有 gomock 生成物和 collaborator expectation 使用方式，只迁移断言表达。
- 移除不必要的机械 `Fail` / `Failf` 替换和旧兼容 helper，不新增旧 permission 字段、旧 route scanner 输出、旧 watcher 签名或旧授权白名单兼容断言。
- **BREAKING**: permission 测试不再接受为旧接口、旧输出或旧兼容路径保留的测试断言形态；本项只影响测试约束，不改变运行时 API 或生产行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 增加 permission feature 测试断言规范要求，覆盖权限目录、授权、policy sync、route diff、HTTP boundary、Casbin adapter、PostgreSQL store、Redis watcher 和 metrics 测试的断言表达约束。

## Impact

- 影响测试代码：`user-service/internal/features/permission/**/*_test.go`。
- 影响测试依赖使用：在 permission 测试中增加或补齐 `github.com/stretchr/testify/require`，必要时使用 `github.com/stretchr/testify/assert`。
- 不影响生产代码、HTTP API、OpenAPI、Casbin model、policy sync、Redis watcher 行为或 PostgreSQL schema。
- 验证范围包括 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/permission --glob '*_test.go'`、`rg "github.com/stretchr/testify/(require|assert)" user-service/internal/features/permission --glob '*_test.go'`、`go test ./user-service/internal/features/permission/...` 和 `openspec validate standardize-permission-test-assertions-no-compat`。
