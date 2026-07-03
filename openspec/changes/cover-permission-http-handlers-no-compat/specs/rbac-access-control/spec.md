## ADDED Requirements

### Requirement: Permission HTTP boundary 测试覆盖

permission feature 的 HTTP boundary 测试 MUST 直接覆盖权限目录生命周期、用户有效权限查询和 route diff controller。测试 MUST 固定请求绑定、input preparer、application command/query port 调用、错误映射、分页 envelope、有效权限 response 和 route diff response 的当前契约，并 MUST NOT 通过旧权限资源路径、旧 action/resource 字段语义、旧错误 envelope、旧授权绕过、旧 route scanner 输出或兼容 helper 表达预期。

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

#### Scenario: application 错误映射

- **WHEN** permission application command/query port 返回 domain、validation、not found、conflict 或内部错误
- **THEN** permission HTTP boundary 测试 MUST 验证 controller 通过当前 permission HTTP error mapper 映射为对应 HTTP status 和 envelope code
- **AND** 测试 MUST NOT 新增旧错误码、旧 message 或旧 envelope 兼容断言

#### Scenario: 保持 permission HTTP 测试边界

- **WHEN** permission HTTP boundary 测试需要构造 collaborator、请求上下文或响应断言
- **THEN** 测试 MUST 使用现有 gomock 生成物或既有生成入口维护的 mock 表达 application port 调用
- **AND** 测试 MUST NOT 引入 infrastructure store、Ent client、PostgreSQL、Redis、Casbin engine、RBAC seed 或跨 feature adapter 作为 controller 单元测试依赖

#### Scenario: 语义化断言和不保留旧兼容路径

- **WHEN** permission HTTP boundary 测试新增或调整断言
- **THEN** 测试 MUST 优先使用 `testify/require` 和 `Len`、`Greater`、`ErrorContains`、`ElementsMatch`、`JSONEq`、`Regexp` 等更具体语义化断言
- **AND** 测试 MUST NOT 新增机械 `Fail` / `Failf` / `FailNow` / `FailNowf` 替换、旧权限资源路径、旧 action/resource 字段、旧 binding、旧 response envelope、旧授权绕过或旧 helper 兼容断言
