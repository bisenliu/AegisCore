## ADDED Requirements

### Requirement: Keep validators pure and stateless

系统 SHALL 将 `user-services/internal/validators` 限制为无状态、无外部依赖的纯业务校验集合。Validators MUST NOT 依赖 DB、Redis、HTTP client、配置中心或其他外部系统；依赖持久化状态或外部状态的业务检查 MUST 位于 service 编排中，并通过明确 port 调用。

#### Scenario: Validate username and password format locally
- **Given** controller 需要校验用户名格式、密码强度、分页边界或输入字符串规范化
- **When** 这些校验不需要访问 DB、Redis 或外部系统
- **Then** 校验 MAY 位于 `user-services/internal/validators`
- **Then** 校验函数 SHOULD 保持纯函数化、可单测、可复用

#### Scenario: Keep uniqueness checks in service
- **Given** 创建用户、登录或绑定邮箱流程需要判断用户名是否已存在、邮箱是否已注册或 session 是否有效
- **When** 该判断依赖 PostgreSQL、Redis 或外部系统
- **Then** `validators` MUST NOT 直接访问数据源执行该检查
- **Then** service MUST 通过最小 port 编排该检查

### Requirement: Separate HTTP request DTOs from application commands

系统 SHALL 明确区分 HTTP 协议层输入和应用层业务输入。能力内 `api/request.go` MUST 负责 HTTP 协议适配，包括 JSON tag、form tag、uri tag、validate tag 和 binding tag；能力内 `commands.go` MUST 负责纯净的业务 command/query，不应携带 Gin、HTTP 或 binding 语义。Controller MUST 负责把 request DTO 映射为 command/query 后再调用 service。

#### Scenario: Map create request to command in controller
- **Given** 调用方提交创建用户 JSON 请求
- **When** controller 完成 request DTO 绑定和校验
- **Then** controller MUST 构造用户能力的 CreateUserCommand
- **Then** controller MAY 在映射时补充 ClientIP、UserAgent、TraceID 或 OperatorID 等 HTTP 上下文信息
- **Then** service MUST 接收 command/query，而不是直接接收 HTTP request DTO

#### Scenario: Reject transport DTO in service
- **Given** 开发者修改用户或认证 service
- **When** service 方法签名准备直接依赖 Gin context、`http.Request` 或能力 `api/request.go` 中的 request DTO
- **Then** 实现 MUST 被视为违反应用层边界
- **Then** controller MUST 先完成 DTO 到 command/query 的映射，再调用 service

#### Scenario: Keep tags out of commands
- **Given** 开发者定义 CreateUserCommand、LoginCommand 或 ListUsersQuery
- **When** 这些类型位于能力根包的 `commands.go`
- **Then** 字段 MUST NOT 携带 JSON、form、uri、binding 或 validate tag
- **Then** command/query MUST 表达业务输入，而不是传输层协议格式
