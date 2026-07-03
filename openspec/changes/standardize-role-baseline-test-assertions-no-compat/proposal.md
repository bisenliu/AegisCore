## Why

role feature 和 shared RBAC baseline 的历史 Go 测试仍混用 `t.Fatal`、`t.Errorf`、手写 if 断言和少量语义化断言，导致失败信息、前置条件失败处理和诊断风格在 command/query/seed、store adapter、HTTP boundary 与 baseline catalog 测试之间不一致。

本次变更在不修改角色、用户角色、角色权限、RBAC seed 或 baseline 生产行为的前提下，统一测试断言规范，并明确不为旧 role 字段、旧 binding 行为、旧 baseline catalog 或旧 fake/helper 形态保留兼容断言。

## What Changes

- 将 `user-service/internal/features/role/**/*_test.go` 和 `user-service/internal/shared/rbacbaseline/**/*_test.go` 中的历史手写断言迁移为 `testify/require` 语义化断言。
- 对多字段响应或 baseline catalog 校验中互相独立的字段失败收集，允许按 `docs/TESTING.md` 使用 `testify/assert`。
- 覆盖 role application command/query/seed、domain、transport/http、infrastructure/postgres、RoleStore/UserRoleStore/RolePermissionStore 和 shared RBAC baseline catalog 测试。
- 保持已有 gomock 生成物和 collaborator expectation 使用方式，只迁移断言表达，不回退为手写 store/notifier double。
- 不新增旧 role 字段、旧 binding 行为、旧 baseline 兼容断言、旧 fake 兼容 helper、机械 `Fail` / `Failf` 替换或旧手写断言兼容层。
- **BREAKING**: role 与 baseline 测试不再接受为旧接口、旧 catalog 或旧测试 double 保留的兼容断言形态；本项只影响测试约束，不改变运行时 API 或生产行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 增加 role feature 和 shared RBAC baseline 测试断言规范要求，覆盖角色管理、角色权限绑定、用户角色绑定、RBAC seed、HTTP boundary、PostgreSQL store 和 baseline catalog 测试的断言表达约束。

## Impact

- 影响测试代码：`user-service/internal/features/role/**/*_test.go` 和 `user-service/internal/shared/rbacbaseline/**/*_test.go`。
- 影响测试依赖使用：在 role 与 baseline 测试中增加或补齐 `github.com/stretchr/testify/require`，必要时使用 `github.com/stretchr/testify/assert`。
- 不影响生产代码、HTTP API、OpenAPI、Casbin model、policy sync、RBAC seed 语义、超级管理员绑定、PostgreSQL schema、Atlas migration 或部署资产。
- 验证范围包括 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/role user-service/internal/shared/rbacbaseline --glob '*_test.go'`、`rg "github.com/stretchr/testify/(require|assert)" user-service/internal/features/role user-service/internal/shared/rbacbaseline --glob '*_test.go'`、`go test ./user-service/internal/features/role/... ./user-service/internal/shared/rbacbaseline/...` 和 `openspec validate standardize-role-baseline-test-assertions-no-compat`。
