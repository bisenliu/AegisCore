## ADDED Requirements

### Requirement: 角色与绑定错误应用错误渲染

系统 MUST 将角色目录、用户角色绑定和角色权限绑定能力中的稳定业务错误表达为可由共享 response helper 直接渲染的应用错误，并保持 role HTTP 边界无专用 sentinel-to-HTTP 兼容映射。

#### Scenario: 角色已存在渲染为冲突响应

- **WHEN** 角色创建或更新流程返回 `roledomain.ErrRoleAlreadyExists`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前角色已存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_already_exists`

#### Scenario: 角色不存在渲染为未找到响应

- **WHEN** 角色详情查询、更新、启停、用户角色绑定或角色权限绑定流程返回 `roledomain.ErrRoleNotFound`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前角色不存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_not_found`

#### Scenario: 角色输入无效渲染为 validation 响应

- **WHEN** 角色 domain validation 返回 `roledomain.ErrRoleInvalid`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `400 Bad Request` 和共享 validation 业务 code
- **AND** 响应 message MUST 使用当前角色输入无效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_invalid`

#### Scenario: 系统角色保护渲染为冲突响应

- **WHEN** 角色更新或启停流程返回 `roledomain.ErrSystemRoleProtected`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前系统角色保护中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `system_role_protected`

#### Scenario: 停用角色渲染为冲突响应

- **WHEN** 用户角色绑定流程返回 `roledomain.ErrRoleInactive`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前角色已停用中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_inactive`

#### Scenario: 用户角色绑定已存在渲染为冲突响应

- **WHEN** 用户角色增量绑定流程返回 `roledomain.ErrUserRoleAlreadyExists`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前用户角色绑定已存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `user_role_already_exists`

#### Scenario: 用户角色绑定不存在渲染为未找到响应

- **WHEN** 用户角色解绑流程返回 `roledomain.ErrUserRoleNotFound`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前用户角色绑定不存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `user_role_not_found`

#### Scenario: 角色权限绑定已存在渲染为冲突响应

- **WHEN** 角色权限增量绑定流程返回 `roledomain.ErrRolePermissionAlreadyExists`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前角色权限绑定已存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_permission_already_exists`

#### Scenario: 角色权限绑定不存在渲染为未找到响应

- **WHEN** 角色权限解绑或绑定查询流程返回 `roledomain.ErrRolePermissionNotFound`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前角色权限绑定不存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_permission_not_found`

#### Scenario: 跨 feature 不存在错误透传

- **WHEN** 用户角色绑定流程返回 `identity.ErrUserNotFound` 或角色权限绑定流程返回 `permissiondomain.ErrPermissionNotFound`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 直接透传渲染失败响应
- **AND** 用户不存在 MUST 返回 `404 Not Found` 和用户身份错误自身携带的共享未找到业务 code 与公开 message
- **AND** 权限不存在 MUST 返回 `404 Not Found` 和权限目录错误自身携带的共享未找到业务 code 与公开 message
- **AND** role HTTP transport MUST NOT 复制 identity 或 permission 错误映射

#### Scenario: 角色业务错误保留 errors.Is 语义

- **WHEN** role feature、seed 或测试需要判断角色目录、用户角色绑定或角色权限绑定错误
- **THEN** `errors.Is` 对直接返回的角色应用错误和被包装后的角色应用错误 MUST 继续支持正确匹配
- **AND** 系统 MUST NOT 为 role HTTP transport 保留 `toRoleHTTPError` 或等价兼容函数

### Requirement: 角色 HTTP transport 统一错误出口

role HTTP transport MUST 对业务 command/query 返回错误使用共享 `response.Fail` 入口，避免在 transport 层重复维护角色、用户角色绑定、角色权限绑定、identity 或 permission 错误到 HTTP 响应的映射。

#### Scenario: 角色目录 controller 业务错误

- **WHEN** `ListRoles`、`CreateRole`、`GetRole`、`UpdateRole` 或 `SetRoleStatus` controller 调用角色 command/query use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用角色专用错误 mapper

#### Scenario: 用户角色 controller 业务错误

- **WHEN** `ListUserRoles`、`ReplaceUserRoles`、`AddUserRole` 或 `RemoveUserRole` controller 调用用户角色 command/query use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用角色专用错误 mapper

#### Scenario: 角色权限 controller 业务错误

- **WHEN** `ListRolePermissions`、`ReplaceRolePermissions`、`AddRolePermission` 或 `RemoveRolePermission` controller 调用角色权限 command/query use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用角色专用错误 mapper

#### Scenario: role transport 不保留跨 feature 错误 mapper

- **WHEN** role HTTP transport 接收来自 identity、permission 或 role domain 的应用错误
- **THEN** controller MUST 通过共享 response helper 渲染错误自身携带的 HTTP status、code 和 message
- **AND** role HTTP transport MUST NOT 新增或保留将 role、identity 或 permission sentinel error 转换为 HTTP 应用错误的 mapper

### Requirement: 角色 HTTP boundary 测试覆盖应用错误直通

role feature 的 HTTP boundary 测试 MUST 覆盖角色目录、用户角色绑定和角色权限绑定 controller 的应用错误直通渲染。测试 MUST 固定请求绑定、input preparer、application command/query port 调用、错误直通渲染、角色 response 和权限 response 的当前契约，并 MUST NOT 通过旧错误 mapper 表达预期。

#### Scenario: 角色目录错误直通渲染

- **WHEN** role HTTP 测试覆盖角色创建、详情、更新或启停 handler 的业务错误
- **THEN** 测试 MUST 验证 controller 通过 `response.Fail(c, err)` 渲染角色已存在、角色不存在、角色输入无效、系统角色保护和未知内部错误响应
- **AND** 测试 MUST NOT 依赖 `toRoleHTTPError` 或等价兼容函数

#### Scenario: 用户角色错误直通渲染

- **WHEN** role HTTP 测试覆盖用户角色替换、增量绑定或解绑 handler 的业务错误
- **THEN** 测试 MUST 验证 controller 通过 `response.Fail(c, err)` 渲染角色不存在、角色停用、用户角色绑定已存在、用户角色绑定不存在、用户不存在和未知内部错误响应
- **AND** 测试 MUST NOT 在 role transport 层复制 identity 错误映射

#### Scenario: 角色权限错误直通渲染

- **WHEN** role HTTP 测试覆盖角色权限替换、增量绑定或解绑 handler 的业务错误
- **THEN** 测试 MUST 验证 controller 通过 `response.Fail(c, err)` 渲染角色不存在、权限不存在、角色权限绑定已存在、角色权限绑定不存在和未知内部错误响应
- **AND** 测试 MUST NOT 在 role transport 层复制 permission 错误映射

#### Scenario: 保持 role HTTP 测试边界

- **WHEN** role HTTP boundary 测试需要构造 collaborator、请求上下文或响应断言
- **THEN** 测试 MUST 使用现有 gomock 生成物或既有生成入口维护的 mock 表达 application port 调用
- **AND** 测试 MUST NOT 引入 infrastructure store、Ent client、PostgreSQL、Redis、Casbin engine、RBAC seed 或跨 feature adapter 作为 controller 单元测试依赖

