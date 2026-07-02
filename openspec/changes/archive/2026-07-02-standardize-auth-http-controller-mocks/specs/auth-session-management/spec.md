## MODIFIED Requirements

### Requirement: 认证 HTTP 边界

系统 MUST 将公开认证路由和受保护认证路由分开挂载，并通过共享认证中间件保护需要 bearer token 的接口。认证 HTTP 边界 MUST 区分凭据认证失败和认证服务临时不可用。认证 HTTP controller 测试 MUST 使用 feature-local `gomock` 生成 mock 表达 use case 调用契约，不得保留手写 `stubAuthUseCases` 兼容入口。

#### Scenario: 公开登录路由

- **WHEN** 调用方访问登录或刷新等公开认证入口
- **THEN** 系统 MUST 允许请求进入认证 controller 并在业务层完成凭证校验

#### Scenario: 受保护认证路由

- **WHEN** 调用方访问退出、修改密码或其他受保护认证入口
- **THEN** 系统 MUST 先通过 JWT、auth config 和 token version validator 校验

#### Scenario: 无效 bearer token

- **WHEN** 受保护认证路由收到缺失、过期、格式错误或签名无效的 bearer token
- **THEN** 系统 MUST 在进入业务处理前拒绝请求

#### Scenario: 登录 KDF busy HTTP 响应

- **WHEN** 登录 use case 返回 `password.ErrPasswordKDFBusy`
- **THEN** 认证 HTTP 边界 MUST 返回 `503 Service Unavailable`
- **AND** 响应 envelope MUST 使用服务不可用错误分类和认证服务繁忙消息
- **AND** OpenAPI MUST 声明登录接口可能返回 503

#### Scenario: controller 测试验证 use case 调用契约

- **WHEN** 认证 HTTP controller 测试覆盖登录、刷新、改密、退出当前会话或退出全部会话流程
- **THEN** 测试 MUST 使用 `auth/transport/http` 测试包内的 `gomock` 生成 mock 设置 use case expectation
- **AND** 测试 MUST 通过 expectation、matcher 或 `DoAndReturn` 验证命令归一化、client context 注入和错误映射
- **AND** 测试 MUST NOT 通过手写 `stubAuthUseCases` 或只服务于该 stub 的状态字段表达调用契约
