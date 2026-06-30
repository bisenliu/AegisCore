## Why

RBAC 授权服务在解析认证用户 ID 失败时返回 `(false, nil)`，会把非法 subject 与正常权限拒绝折叠为同一种结果。该行为降低了错误语义清晰度和内部可观测性，也会增加授权接口未来被复用时误判故障的风险。

## What Changes

- 调整 permission 授权应用层在用户 ID 非法时的错误语义：必须返回明确错误，而不是静默返回不允许。
- HTTP 授权适配层必须继续保持 fail closed，并将非法 subject 映射为认证上下文无效或其他明确错误路径，不得误报为普通 RBAC 策略拒绝。
- 更新相关单元测试，覆盖非法用户 ID 不调用底层 engine、返回明确错误、HTTP 层不会把该错误吞并为普通权限拒绝。
- 不改变外部 API 路由、请求/响应结构、数据库 schema、OpenAPI 契约或 Casbin policy 数据模型。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: RBAC 授权判断必须保留非法认证 subject 与策略拒绝的错误语义差异，同时保持安全拒绝行为。

## Impact

- 影响代码：`user-service/internal/features/permission/application/authorization/authorization.go`、`user-service/internal/features/permission/transport/http/authorization.go` 及其测试。
- 影响行为：非法认证用户 ID 不再在应用层表现为无错误的 `allowed=false`，调用方可通过错误区分解析失败和权限拒绝。
- 安全影响：授权路径继续 fail closed，不会放行非法 subject。
- API 影响：无路由、DTO、OpenAPI 或 HTTP 成功响应结构变化；错误响应可能从普通 `403 Forbidden` 调整为更符合认证上下文无效的失败路径。
- 数据和部署影响：无数据库 schema、migration、部署资产或外部依赖变化。
