## Why

role HTTP boundary 当前缺少直接 controller 级测试覆盖，角色生命周期、角色权限绑定和用户角色绑定请求只能间接依赖 input 或更高层流程验证。补齐边界测试可以固定请求绑定、input preparer、application port 调用、错误映射和 response envelope 的当前契约，避免后续通过旧字段、旧绑定形态或兼容响应路径绕开角色管理边界。

本次变更不修改生产 API、OpenAPI、RBAC 授权语义或数据库结构，只补齐 role HTTP 边界的可回归测试约束。

## What Changes

- 为 `user-service/internal/features/role/transport/http` 新增或补充 controller 级测试，覆盖角色列表、创建、详情、更新、启停、角色权限绑定和用户角色绑定 HTTP handler。
- 测试覆盖成功响应、请求绑定失败、UUID/cursor 解析失败、application command/query 错误映射和 response envelope 数据映射。
- 使用现有 gomock collaborator 表达 command/query port 调用，保持 HTTP boundary 只依赖 application interface，不引入 infrastructure store 或跨 feature adapter。
- 使用 `testify/require` 或必要的 `assert` 表达语义化断言，并保持 mock 生成物由既有 `go generate` 入口维护。
- **BREAKING**: role HTTP boundary 测试不得新增旧 role 字段、旧请求字段别名、旧 binding 行为、旧 envelope 形态或旧错误码兼容断言路径；本项只影响测试约束，不改变运行时 API 或生产行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 增加 role HTTP boundary 测试覆盖要求，固定角色、角色权限绑定和用户角色绑定 HTTP handler 的请求绑定、输入归一化、application port 调用、错误映射和 envelope 响应契约。

## Impact

- 影响测试代码：`user-service/internal/features/role/transport/http/**/*_test.go`。
- 可能影响测试生成物：如需新增 controller collaborator mock，使用既有 `mock_generate.go` / `go generate` 入口更新 `mock_*_test.go`。
- 不影响生产代码、HTTP API、OpenAPI、Casbin model、policy sync、RBAC seed、PostgreSQL schema、Atlas migration 或部署资产。
- 验证范围包括 `go test ./user-service/internal/features/role/transport/http`、`go test ./user-service/internal/features/role/...`、`openspec validate cover-role-http-boundary-no-compat`，以及必要的 `rg` 核查确保未新增旧字段、旧 binding 或旧兼容断言。
