## MODIFIED Requirements

### Requirement: Provide service-local validation for user request rules
用户服务 MUST 在 feature-local HTTP transport 边界保留可维护的校验器集合，用于承载用户服务特定的 HTTP request DTO 清洗、基础校验、解析、转换和复杂请求规则，并 MUST 复用 `common/validation` 和 `common/contract/response` 的共享能力。用户服务特定规则 MUST NOT 上移到 `common`，除非多个服务存在稳定复用需求。用户和认证 HTTP 请求校验器 MUST 分别位于 `user-services/internal/features/user/transport/http/validation.go` 与 `user-services/internal/features/auth/transport/http/validation.go`，并可在同一 package 内按文件继续拆分。

#### Scenario: Locate user HTTP validation
- **Given** controller 需要校验用户创建、用户查询或用户列表的 HTTP request DTO
- **When** 规则只依赖请求字段、标准库、feature API DTO、feature domain 枚举或共享校验/响应原语
- **Then** 规则 MUST 位于 `user-services/internal/features/user/transport/http`
- **Then** 规则 MUST NOT 位于全局 `user-services/internal/validators`

#### Scenario: Locate auth HTTP validation
- **Given** controller 需要校验登录、刷新、强制改密、退出当前设备或退出全部设备的 HTTP request DTO
- **When** 规则只依赖请求字段、标准库、feature API DTO、feature domain 类型或共享校验/响应原语
- **Then** 规则 MUST 位于 `user-services/internal/features/auth/transport/http`
- **Then** 规则 MUST NOT 位于全局 `user-services/internal/validators`

#### Scenario: Reuse shared validation primitives
- **Given** 用户请求需要绑定 Gin URI、query、JSON 或 form 参数并执行结构体校验
- **When** Controller 处理请求
- **Then** Controller MUST 优先复用 `common/validation` 和 `common/http/ginvalidation` 的共享绑定、结构体校验、字段明细和错误归一化能力
- **Then** feature-local HTTP validation MUST NOT 重复实现响应信封、中间件或基础设施能力

### Requirement: Keep validators pure and stateless
系统 SHALL 将 feature-local HTTP validation 限制为无状态、无外部依赖的请求边界校验集合。HTTP validation MUST NOT 依赖 DB、Redis、HTTP client、配置中心、Ent client、feature infra adapter 或其他外部系统；依赖持久化状态或外部状态的业务检查 MUST 位于 app service 编排中，并通过明确 port 调用。

#### Scenario: Validate username and password format locally
- **Given** controller 需要校验用户名格式、密码强度、分页边界、UUID 格式或输入字符串规范化
- **When** 这些校验不需要访问 DB、Redis 或外部系统
- **Then** 校验 MAY 位于对应 feature 的 `transport/http/validation.go`
- **Then** 校验函数 SHOULD 保持纯函数化、可单测、可复用

#### Scenario: Keep uniqueness checks in service
- **Given** 创建用户、登录或绑定邮箱流程需要判断用户名是否已存在、邮箱是否已注册或 session 是否有效
- **When** 该判断依赖 PostgreSQL、Redis 或外部系统
- **Then** feature-local HTTP validation MUST NOT 直接访问数据源执行该检查
- **Then** app service MUST 通过最小 port 编排该检查

#### Scenario: Keep HTTP validation out of app service
- **Given** 开发者修改 `features/<feature>/app` 中的 service 或 command/query 类型
- **When** 输入规则只用于 Gin binding 后的 HTTP request DTO 清洗或转换
- **Then** 该规则 MUST 位于 `features/<feature>/transport/http`
- **Then** app command/query MUST NOT 携带 JSON、form、uri、binding 或 validate tag

### Requirement: Separate HTTP request DTOs from application commands
系统 SHALL 明确区分 HTTP 协议层输入和应用层业务输入。能力内 `api/request.go` MUST 负责 HTTP 协议适配，包括 JSON tag、form tag、uri tag、validate tag 和 binding tag；能力内 `app/commands.go` 或 `app/queries.go` MUST 负责纯净的业务 command/query，不应携带 Gin、HTTP 或 binding 语义。`transport/http` Controller MUST 负责把 request DTO 映射为 command/query 后再调用 app service。

#### Scenario: Map create request to command in controller
- **Given** 调用方提交创建用户 JSON 请求
- **When** `transport/http` controller 完成 request DTO 绑定和校验
- **Then** controller MUST 构造用户能力的 `app.CreateUserCommand`
- **Then** controller MAY 在映射时补充 ClientIP、UserAgent、TraceID 或 OperatorID 等 HTTP 上下文信息
- **Then** service MUST 接收 command/query，而不是直接接收 HTTP request DTO

#### Scenario: Reject transport DTO in service
- **Given** 开发者修改用户或认证 service
- **When** service 方法签名准备直接依赖 Gin context、`http.Request` 或能力 `api/request.go` 中的 request DTO
- **Then** 实现 MUST 被视为违反应用层边界
- **Then** controller MUST 先完成 DTO 到 command/query 的映射，再调用 service

#### Scenario: Keep tags out of commands
- **Given** 开发者定义 CreateUserCommand、LoginCommand 或 ListUsersQuery
- **When** 这些类型位于能力 `app` 包
- **Then** 字段 MUST NOT 携带 JSON、form、uri、binding 或 validate tag
- **Then** command/query MUST 表达业务输入，而不是传输层协议格式
