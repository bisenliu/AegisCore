## Why

权限目录相关错误当前以普通 sentinel error 表达，HTTP 边界需要维护 `toPermissionHTTPError` 这类仅服务权限模块的错误翻译逻辑。将权限不存在、权限已存在、权限输入无效和系统权限保护迁移为携带 `Kind`、`Reason`、`Code`、`Message` 的应用错误，可以让业务层保留 `errors.Is` 语义，同时让权限 controller 统一通过共享 `response.Fail(c, err)` 渲染错误。

## What Changes

- 将 `user-service/internal/features/permission/domain` 中的权限目录错误定义为应用错误，并为每类权限错误提供稳定 `Reason`、`Kind`、`Code` 和中文公开消息。
- 调整 permission command、query、authorization HTTP transport 和相关测试，使业务调用失败直接交给 `response.Fail(c, err)` 渲染。
- 删除 `user-service/internal/features/permission/transport/http` 中仅用于权限错误翻译的 mapper 逻辑，不保留 `toPermissionHTTPError` 或等价兼容函数。
- 更新 RBAC 访问控制规格，明确权限已存在、权限不存在、权限输入无效和系统权限保护的 HTTP 渲染行为，以及权限 HTTP controller 的统一错误出口。
- 更新共享平台 primitive 规格，明确 feature-local domain 可以定义可由共享 response helper 直接渲染、同时保留 `errors.Is` 判断语义的应用错误，但不得新增跨模块权限错误映射注册表。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 权限目录错误的公开 HTTP 渲染方式、权限 HTTP controller 错误处理边界和 permission HTTP boundary 测试契约发生变化。
- `shared-platform-primitives`: feature-local domain 错误需要复用共享应用错误契约表达，并由共享 response helper 直接渲染。

## Impact

- 影响代码：`user-service/internal/features/permission/domain/errors.go`，permission application command/query/authorization，permission HTTP transport 和相关测试。
- API 行为：权限已存在继续返回 `409 Conflict`，权限不存在继续返回 `404 Not Found`，权限输入无效继续返回 `400 Bad Request` validation，系统权限保护继续返回 `409 Conflict`；权限目录 API 路由、请求 DTO、成功响应 data 结构、Casbin policy sync 和 route diff 业务逻辑不变。
- 共享契约：复用现有 `common/contract/errors` 与 `common/http/response` 应用错误渲染能力，不新增跨模块权限错误映射注册表。
- 数据库与部署：不修改 Ent schema、Atlas migration、OpenAPI 路由、部署资产或运行时配置。
