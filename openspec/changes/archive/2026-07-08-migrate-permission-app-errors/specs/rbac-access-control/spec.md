## ADDED Requirements

### Requirement: 权限目录错误应用错误渲染

系统 MUST 将权限目录能力中的权限已存在、权限不存在、权限输入无效和系统权限保护错误表达为可由共享 response helper 直接渲染的应用错误，并保持权限 HTTP 边界无专用 sentinel-to-HTTP 兼容映射。

#### Scenario: 权限已存在渲染为冲突响应

- **WHEN** 权限创建或更新流程返回 `permissiondomain.ErrPermissionAlreadyExists`
- **THEN** 权限 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前权限已存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `permission_already_exists`

#### Scenario: 权限不存在渲染为未找到响应

- **WHEN** 权限详情查询、更新或启停流程返回 `permissiondomain.ErrPermissionNotFound`
- **THEN** 权限 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前权限不存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `permission_not_found`

#### Scenario: 权限输入无效渲染为 validation 响应

- **WHEN** 权限 domain validation 返回 `permissiondomain.ErrPermissionInvalid`
- **THEN** 权限 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `400 Bad Request` 和共享 validation 业务 code
- **AND** 响应 message MUST 使用当前权限输入无效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `permission_invalid`

#### Scenario: 系统权限保护渲染为冲突响应

- **WHEN** 权限更新流程返回 `permissiondomain.ErrSystemPermissionProtected`
- **THEN** 权限 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前系统权限保护中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `system_permission_protected`

#### Scenario: 权限业务错误保留 errors.Is 语义

- **WHEN** permission feature 或测试需要判断权限已存在、权限不存在、权限输入无效或系统权限保护错误
- **THEN** `errors.Is` 对直接返回的权限应用错误和被包装后的权限应用错误 MUST 继续支持正确匹配
- **AND** 系统 MUST NOT 为 permission HTTP transport 保留 `toPermissionHTTPError` 或等价兼容函数

### Requirement: 权限 HTTP transport 统一错误出口

permission HTTP transport MUST 对业务 command/query 返回错误使用共享 `response.Fail` 入口，避免在 transport 层重复维护权限目录错误到 HTTP 响应的映射。授权中间件错误处理 MUST 继续使用共享 response helper，且不得复用或新增权限目录错误 mapper。

#### Scenario: 权限创建业务错误

- **WHEN** `CreatePermission` controller 调用权限创建 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限详情查询业务错误

- **WHEN** `GetPermission` controller 调用权限查询 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限列表查询业务错误

- **WHEN** `ListPermissions` controller 调用权限列表 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限更新业务错误

- **WHEN** `UpdatePermission` controller 调用权限更新 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限启停业务错误

- **WHEN** `EnablePermission` 或 `DisablePermission` controller 调用权限启停 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限有效权限与 route diff 业务错误

- **WHEN** `ListEffectivePermissions` 或 `DiffRoutes` controller 调用权限 query use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 授权 HTTP transport 不使用权限目录 mapper

- **WHEN** permission HTTP 授权中间件处理缺失认证、策略拒绝或授权执行错误
- **THEN** 授权中间件 MUST 使用共享 response helper 渲染当前未认证、禁止访问或内部错误响应
- **AND** 授权中间件 MUST NOT 调用 `toPermissionHTTPError` 或任何权限目录错误 mapper

## MODIFIED Requirements

### Requirement: Permission HTTP boundary 测试覆盖

permission feature 的 HTTP boundary 测试 MUST 直接覆盖权限目录生命周期、用户有效权限查询和 route diff controller。测试 MUST 固定请求绑定、input preparer、application command/query port 调用、错误直通渲染、分页 envelope、有效权限 response 和 route diff response 的当前契约，并 MUST NOT 通过旧权限资源路径、旧 action/resource 字段语义、旧错误 envelope、旧授权绕过、旧 route scanner 输出或兼容 helper 表达预期。

#### Scenario: 权限目录 handler 成功路径

- **WHEN** permission HTTP 测试覆盖权限列表、创建、详情、更新、启用和停用 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用对应 permission application command/query port，并传入由当前 URI、query 和 JSON body 归一化得到的 command/query
- **AND** 测试 MUST 验证成功响应使用当前 response envelope、HTTP status、分页信息和 permission response 字段映射

#### Scenario: 用户有效权限 handler 成功路径

- **WHEN** permission HTTP 测试覆盖查询用户有效权限 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用当前 permission query port，并传入当前 user ID
- **AND** 测试 MUST 验证成功响应使用当前 response envelope 和有效权限 response 字段映射

#### Scenario: route diff handler 成功路径

- **WHEN** permission HTTP 测试覆盖 route diff handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用当前 permission query port 获取 route diff 结果
- **AND** 测试 MUST 验证成功响应使用当前 response envelope 和 missing、stale、mismatch 诊断字段映射

#### Scenario: 请求绑定和输入解析失败

- **WHEN** permission HTTP controller 收到非法 URI UUID、非法 cursor、非法 query 参数、非法 JSON body 或缺失必填字段
- **THEN** 测试 MUST 验证请求在 HTTP boundary 被拒绝并返回当前 bad request 或 validation failed envelope
- **AND** 测试 MUST 验证对应 application command/query port 未被调用

#### Scenario: application 错误直通渲染

- **WHEN** permission application command/query port 返回 domain、validation、not found、conflict 或内部错误
- **THEN** permission HTTP boundary 测试 MUST 验证 controller 通过 `response.Fail(c, err)` 渲染对应 HTTP status 和 envelope code
- **AND** 测试 MUST 覆盖权限已存在、权限不存在、权限输入无效、系统权限保护和未知内部错误响应
- **AND** 测试 MUST NOT 新增旧错误码、旧 message、旧 envelope 或权限专用错误 mapper 兼容断言

#### Scenario: 保持 permission HTTP 测试边界

- **WHEN** permission HTTP boundary 测试需要构造 collaborator、请求上下文或响应断言
- **THEN** 测试 MUST 使用现有 gomock 生成物或既有生成入口维护的 mock 表达 application port 调用
- **AND** 测试 MUST NOT 引入 infrastructure store、Ent client、PostgreSQL、Redis、Casbin engine、RBAC seed 或跨 feature adapter 作为 controller 单元测试依赖

#### Scenario: 语义化断言和不保留旧兼容路径

- **WHEN** permission HTTP boundary 测试新增或调整断言
- **THEN** 测试 MUST 优先使用 `testify/require` 和 `Len`、`Greater`、`ErrorContains`、`ElementsMatch`、`JSONEq`、`Regexp` 等更具体语义化断言
- **AND** 测试 MUST NOT 新增机械 `Fail` / `Failf` / `FailNow` / `FailNowf` 替换、旧权限资源路径、旧 action/resource 字段、旧 binding、旧 response envelope、旧授权绕过或旧 helper 兼容断言
