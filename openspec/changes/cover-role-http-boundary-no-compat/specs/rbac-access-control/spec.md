## ADDED Requirements

### Requirement: Role HTTP boundary 测试覆盖

role feature 的 HTTP boundary 测试 MUST 直接覆盖角色生命周期、角色权限绑定和用户角色绑定 controller。测试 MUST 固定请求绑定、input preparer、application command/query port 调用、错误映射和 response envelope 的当前契约，并 MUST NOT 通过旧 role 字段、旧请求字段别名、旧 binding 行为、旧 envelope 形态、旧错误码或兼容 helper 表达预期。

#### Scenario: 角色生命周期 handler 成功路径

- **WHEN** role HTTP 测试覆盖角色列表、创建、详情、更新和启停 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用对应 role application command/query port，并传入由当前 URI、query 和 JSON body 归一化得到的 command/query
- **AND** 测试 MUST 验证成功响应使用当前 response envelope、HTTP status 和 role response 字段映射

#### Scenario: 角色权限绑定 handler 成功路径

- **WHEN** role HTTP 测试覆盖查询、替换、新增和移除角色权限绑定 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用对应 role application command/query port，并传入当前 role ID、permission ID 或 permission ID 集合
- **AND** 测试 MUST 验证成功响应使用当前 response envelope、HTTP status 和 permission response 字段映射

#### Scenario: 用户角色绑定 handler 成功路径

- **WHEN** role HTTP 测试覆盖查询、替换、新增和移除用户角色绑定 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用对应 role application command/query port，并传入当前 user ID、role ID 或 role ID 集合
- **AND** 测试 MUST 验证成功响应使用当前 response envelope、HTTP status 和 role response 字段映射

#### Scenario: 请求绑定和输入解析失败

- **WHEN** role HTTP controller 收到非法 URI UUID、非法 cursor、非法 query 参数、非法 JSON body 或缺失必填字段
- **THEN** 测试 MUST 验证请求在 HTTP boundary 被拒绝并返回当前 bad request 或 validation failed envelope
- **AND** 测试 MUST 验证对应 application command/query port 未被调用

#### Scenario: application 错误映射

- **WHEN** role application command/query port 返回 domain、validation、not found、conflict 或内部错误
- **THEN** role HTTP boundary 测试 MUST 验证 controller 通过当前 `toRoleHTTPError` 映射为对应 HTTP status 和 envelope code
- **AND** 测试 MUST NOT 新增旧错误码、旧 message 或旧 envelope 兼容断言

#### Scenario: 保持 role HTTP 测试边界

- **WHEN** role HTTP boundary 测试需要构造 collaborator、请求上下文或响应断言
- **THEN** 测试 MUST 使用现有 gomock 生成物或既有生成入口维护的 mock 表达 application port 调用
- **AND** 测试 MUST NOT 引入 infrastructure store、Ent client、PostgreSQL、Casbin engine、RBAC seed 或跨 feature adapter 作为 controller 单元测试依赖

#### Scenario: 不保留旧兼容路径

- **WHEN** role HTTP boundary 测试新增或调整断言
- **THEN** 测试 MUST NOT 新增旧 role 字段、旧 request body 字段别名、旧 binding 行为、旧 response envelope、旧错误码或旧 helper 兼容断言
- **AND** 测试 MUST 使用 `testify/require` 或必要的 `assert` 表达语义化断言，MUST NOT 使用机械 `Fail` / `Failf` 替换来模拟历史手写失败判断
