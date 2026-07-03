## Why

permission HTTP controller 当前多个 handler 缺少直接测试覆盖，权限目录生命周期、用户有效权限查询和 route diff 入口只能依赖 input、mapper 或更高层流程间接约束。补齐 controller 级测试可以固定请求绑定、input preparer、application port 调用、错误映射、分页 envelope 和当前 response 映射，避免后续通过旧权限资源路径、旧 action/resource 语义或旧 envelope 兼容路径绕过权限边界。

本次变更不修改生产 API、OpenAPI、Casbin 授权语义、policy sync、Redis watcher、RBAC seed 或数据库结构，只补齐 permission HTTP boundary 的可回归测试约束。

## What Changes

- 为 `user-service/internal/features/permission/transport/http` 新增或补充 controller 级测试，覆盖权限列表、创建、详情、更新、启停、用户有效权限查询和 route diff HTTP handler。
- 测试覆盖成功响应、请求绑定失败、UUID/cursor/query 解析失败、application command/query 错误映射、分页响应、route diff 响应和有效权限响应。
- 使用现有 gomock collaborator 表达 permission application command/query port 调用，保持 HTTP boundary 测试位于 application interface 边界，不引入 infrastructure store、Casbin engine 或 Redis 依赖。
- 使用 `testify/require` 或必要的 `assert` 表达语义化断言，并优先使用 `Len`、`Greater`、`ErrorContains`、`ElementsMatch`、`JSONEq`、`Regexp` 等更具体断言。
- **BREAKING**: permission HTTP boundary 测试不得新增旧权限资源路径、旧 action/resource 字段语义、旧错误 envelope、旧授权绕过、旧 route scanner 输出或兼容 helper 断言路径；本项只影响测试约束，不改变运行时 API 或生产行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 增加 permission HTTP boundary 测试覆盖要求，固定权限目录、用户有效权限查询和 route diff handler 的请求绑定、输入归一化、application port 调用、错误映射、分页 envelope 和 response 映射契约。

## Impact

- 影响测试代码：`user-service/internal/features/permission/transport/http/**/*_test.go`。
- 可能影响测试生成物：如需新增 controller collaborator mock，使用既有 `mock_generate.go` / `go generate` 入口更新 `mock_*_test.go`。
- 不影响生产代码、HTTP API、OpenAPI 注解或生成物、Casbin model、policy sync、Redis watcher、RBAC seed、PostgreSQL schema、Atlas migration 或部署资产。
- 验证范围包括 `go test -cover ./user-service/internal/features/permission/transport/http`、`go test ./user-service/internal/features/permission/...`、`openspec validate cover-permission-http-handlers-no-compat`，以及必要的 `go tool cover -func` 与 `rg` 核查确保未新增旧字段、旧路径或旧兼容断言。
