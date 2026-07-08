## Why

角色、用户角色绑定和角色权限绑定错误当前仍以普通 sentinel error 表达，角色 HTTP 边界需要维护一组 role、identity、permission 和绑定错误到 HTTP 响应的重复 mapper。将这些错误迁移为携带 `Kind`、`Reason`、`Code`、`Message` 的应用错误，可以保留业务层 `errors.Is` 判断语义，并让角色 controller 统一通过共享 `response.Fail(c, err)` 渲染失败响应。

## What Changes

- 将 `user-service/internal/features/role/domain` 中的角色目录、用户角色绑定和角色权限绑定错误定义为应用错误，并为每类错误提供稳定 `Reason`、`Kind`、`Code` 和中文公开消息。
- 调整 role command、query、seed、transport 和相关测试，使角色业务调用失败直接交给 `response.Fail(c, err)` 渲染。
- 处理 role feature 消费 `identity` 与 `permission` 应用错误后的透传行为，避免 role HTTP transport 再次翻译跨 feature 错误。
- 删除 `user-service/internal/features/role/transport/http` 中仅用于角色错误翻译的 mapper 逻辑，不保留 `toRoleHTTPError` 或等价兼容函数。
- 更新 RBAC 访问控制规格，明确角色已存在、系统角色保护、角色停用、角色或绑定不存在、绑定已存在、用户不存在、权限不存在和角色输入无效的公开 HTTP 渲染行为，以及角色 HTTP controller 的统一错误出口。
- 更新共享平台 primitive 规格，明确 feature-local domain 和服务内 shared kernel 应用错误可以跨 feature 透传给共享 response helper 渲染，但不得在 role transport 复制 identity 或 permission 错误映射。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 角色目录、用户角色绑定和角色权限绑定错误的公开 HTTP 渲染方式，以及角色 HTTP controller 错误处理边界发生变化。
- `shared-platform-primitives`: feature-local domain 错误和服务内 shared identity 错误需要复用共享应用错误契约表达，并允许消费方 feature 通过 `response.Fail` 直接透传渲染。

## Impact

- 影响代码：`user-service/internal/features/role/domain/errors.go`，role application command/query/seed，role HTTP transport 和相关测试；必要时只调整 role feature 对 `identity`、`permission` 应用错误的消费断言。
- API 行为：角色已存在、系统角色保护、角色停用、绑定已存在继续返回 `409 Conflict`；角色、用户、权限或绑定不存在继续返回 `404 Not Found`；角色输入无效继续返回 `400 Bad Request` validation。角色、用户角色、角色权限 API 路由、请求 DTO、成功响应 data 结构、RBAC seed 数据模型和 Casbin policy sync 业务逻辑不变。
- 共享契约：复用现有 `common/contract/errors` 与 `common/http/response` 应用错误渲染能力，不新增跨 feature 错误映射注册表，也不把 role、permission 或 identity 的业务错误映射上移到 `common`。
- 数据库与部署：不修改 Ent schema、Atlas migration、OpenAPI 路由、部署资产或运行时配置。
