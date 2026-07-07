## Why

用户身份相关错误当前以普通 sentinel error 表达，HTTP 边界需要额外维护一层仅服务用户模块的错误翻译逻辑，导致错误来源、业务语义和响应契约分散。将用户不存在、用户已存在迁移为携带 `Kind`、`Reason`、`Code`、`Message` 的应用错误，可以让业务层保留 `errors.Is` 语义，同时让 HTTP controller 统一通过共享 response helper 渲染错误。

## What Changes

- 将 `user-service/internal/shared/identity` 中的用户不存在、用户已存在错误定义为应用错误，并保留 `errors.Is(err, identity.ErrUserNotFound)` 与 `errors.Is(err, identity.ErrUserAlreadyExists)` 的业务判断能力。
- 调整 `user-service/internal/features/user` 的 command、query、transport 和测试，使用户业务错误直接交给 `response.Fail(c, err)` 渲染。
- 删除用户 HTTP transport 中仅用于 sentinel-to-HTTP 转换的 mapper，不保留 `toUserHTTPError` 或等价兼容函数。
- 更新用户身份管理规格，明确用户已存在渲染为冲突响应、用户不存在渲染为未找到响应，且用户 HTTP 边界不再维护用户专用错误翻译层。
- 更新共享平台 primitive 规格，明确服务内 shared kernel 可以定义可被共享 response helper 直接渲染、同时保留 `errors.Is` 判断语义的应用错误，但不得新增跨 feature 用户错误全局映射表。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `user-identity-management`: 用户身份错误的公开 HTTP 渲染方式和用户 HTTP controller 错误处理边界发生变化。
- `shared-platform-primitives`: 服务内 shared kernel 的身份错误需要使用共享应用错误契约表达，并由共享 response helper 直接渲染。

## Impact

- 影响代码：`user-service/internal/shared/identity/errors.go`，`user-service/internal/features/user/application/command`，`user-service/internal/features/user/application/query`，`user-service/internal/features/user/transport/http` 及相关测试。
- API 行为：用户已存在仍返回 `409 Conflict` 和业务冲突 code，用户不存在仍返回 `404 Not Found` 和资源不存在 code；请求 DTO、响应 data 结构和用户业务能力不变。
- 共享契约：复用现有 `common/contract/errors` 与 `common/http/response` 应用错误渲染能力，不新增跨 feature 用户错误映射表。
- 数据库与部署：不修改 Ent schema、Atlas migration、OpenAPI 路由、部署资产或运行时配置。
