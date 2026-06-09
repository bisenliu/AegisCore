# user-service-validation-boundary

## Purpose

用户服务校验边界能力定义用户服务中 Controller、feature-local HTTP validation、app Service 和 feature infra adapter 的职责划分，确保请求级输入清洗与基础校验不混入业务编排或数据访问实现。

## Requirements

### Requirement: Separate request validation from service business logic
用户服务 MUST 将请求级输入清洗和基础参数校验与 Service 业务编排分离。Controller MUST 负责 HTTP 参数提取、DTO 到 command/query 映射和响应输出；feature-local HTTP validation MUST 负责请求字段清洗、格式校验和不依赖持久化状态的跨字段校验；Service MUST 负责业务编排、领域规则和用例输入归一化；feature infra adapter MUST 负责 Ent/PostgreSQL 或 Redis 数据访问。

#### Scenario: Normalize request input before service call
- **Given** 调用方提交包含前后空白的创建用户、列表用户、用户 ID、登录、改密或刷新请求参数
- **When** controller 调用用户服务业务方法前处理请求
- **Then** 请求级字符串裁剪和基础格式校验 MUST 在 Controller 或 feature-local HTTP validation 边界完成
- **Then** Service MUST 接收已经规范化并通过基础校验的输入

#### Scenario: Keep business checks in service
- **Given** 创建用户、登录、改密或刷新请求已经通过请求级校验
- **When** Service 编排用户资料或认证会话流程
- **Then** Service MUST 负责用户名唯一性检查、凭据认证、密码哈希、用户 ID 生成、默认业务状态、JWT claims 校验、session/token version 校验和领域错误映射
- **Then** feature-local HTTP validation MUST NOT 直接访问 infra adapter、Redis、PostgreSQL 或 Ent client 执行持久化状态检查

#### Scenario: Keep database access in store adapters
- **Given** 用户服务需要查询、创建或检查用户记录
- **When** Service 执行业务流程
- **Then** Service MUST 通过 feature app 层声明的最小 port 访问数据
- **Then** Controller 和 feature-local HTTP validation MUST NOT 直接调用 Ent client、Redis client 或具体 infra 实现包

### Requirement: Provide service-local validation for user request rules
用户服务 MUST 在 feature-local HTTP transport 边界保留可维护的校验器集合，用于承载用户服务特定的 HTTP request DTO 清洗、基础校验、解析、转换和复杂请求规则，并 MUST 复用 `common/validation` 和 `common/contract/response` 的共享能力。用户服务特定规则 MUST NOT 上移到 `common`，除非多个服务存在稳定复用需求。用户和认证 HTTP 请求校验器 MUST 分别位于 `user-services/internal/features/user/transport/http/validation.go` 与 `user-services/internal/features/auth/transport/http/validation.go`，并可在同一 package 内按文件继续拆分。

#### Scenario: Locate user HTTP validation
- **Given** controller 需要校验用户创建、用户查询或用户列表的 HTTP request DTO
- **When** 规则只依赖请求字段、标准库、feature API DTO、feature domain 枚举或共享校验/响应原语
- **Then** 规则 MUST 位于 `user-services/internal/features/user/transport/http`
- **Then** 规则 MUST NOT 位于服务级全局 validators 包

#### Scenario: Locate auth HTTP validation
- **Given** controller 需要校验登录、刷新、强制改密、退出当前设备或退出全部设备的 HTTP request DTO
- **When** 规则只依赖请求字段、标准库、feature API DTO、feature domain 类型或共享校验/响应原语
- **Then** 规则 MUST 位于 `user-services/internal/features/auth/transport/http`
- **Then** 规则 MUST NOT 位于服务级全局 validators 包

#### Scenario: Reuse shared validation primitives
- **Given** 用户请求需要绑定 Gin URI、query、JSON 或 form 参数并执行结构体校验
- **When** Controller 处理请求
- **Then** Controller MUST 优先复用 `common/validation` 和 `common/http/ginvalidation` 的共享绑定、结构体校验、字段明细和错误归一化能力
- **Then** feature-local HTTP validation MUST NOT 重复实现响应信封、中间件或基础设施能力

### Requirement: Keep validators pure and stateless
系统 SHALL 将 feature-local HTTP validation 限制为无状态、无外部依赖的请求边界校验集合。HTTP validation MUST NOT 依赖 DB、Redis、HTTP client、配置中心、Ent client、feature infra adapter 或其他外部系统；依赖持久化状态或外部状态的业务检查 MUST 位于 app service 编排中，并通过明确 port 调用。

#### Scenario: Validate username and password format locally
- **Given** controller 需要校验用户名格式、密码强度、分页边界或输入字符串规范化
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

#### Scenario: Validate complex user request rules locally
- **Given** 用户服务接口需要校验用户名、昵称、密码、状态、分页、过滤条件或请求体 token 字段
- **When** 这些规则属于用户服务业务上下文但不需要数据库状态
- **Then** 规则 MUST 位于对应 feature 的 `transport/http` validation 边界
- **Then** Controller MUST 调用该校验边界而不是内联大量复杂字段规则

#### Scenario: Reuse shared validation primitives
- **Given** 用户请求需要绑定 Gin URI、query、JSON 或 form 参数并执行结构体校验
- **When** Controller 处理请求
- **Then** Controller MUST 优先复用 `common/validation` 的共享绑定、结构体校验、字段明细和错误归一化能力
- **Then** feature-local HTTP validation MUST NOT 重复实现响应信封、中间件或基础设施能力

#### Scenario: Keep validators free of persistence checks
- **Given** 用户服务需要检查用户名唯一性、认证凭据、会话状态、权限或其他依赖持久化状态的业务规则
- **When** Service 编排用户资料、认证或团队相关流程
- **Then** feature-local HTTP validation MUST NOT 直接访问 Repository、Ent client、Redis client 或外部服务执行该检查
- **Then** 该检查 MUST 保持在 Service 或其明确依赖的领域编排边界中

#### Scenario: Organize validation by feature transport package
- **Given** 用户服务存在 user、auth、team 或 toll team 等不同领域的 HTTP 输入规则
- **When** 这些规则位于服务内校验器集合中
- **Then** 实现 MUST 位于对应 feature 的 `transport/http` package
- **Then** 包内 MAY 继续按 `validation.go`、`validation_<topic>_test.go` 等文件拆分，允许包含 `Normalize`、`Validate`、`Parse` 等函数

### Requirement: Preserve public API behavior during validation boundary changes
用户服务调整请求校验职责边界时 MUST 保持现有 HTTP 路由、请求字段、响应信封、公开错误语义和用户资料响应字段兼容。

#### Scenario: Validation boundary refactor keeps create API compatible
- **Given** 创建用户请求校验逻辑从 Service 迁移到 Controller 或服务内 validators 层
- **When** 调用方提交合法或非法创建用户请求
- **Then** `POST /api/v1/users` 的路径、请求字段、成功响应字段和失败响应信封 MUST 与迁移前保持兼容
- **Then** 校验失败仍 MUST 通过 `common/contract/response.Envelope` 输出统一 HTTP 400 响应

#### Scenario: Validation boundary refactor keeps query API compatible
- **Given** 查询用户请求校验逻辑从 Service 迁移到 Controller 或服务内 validators 层
- **When** 调用方提交合法或非法查询请求
- **Then** `GET /api/v1/users/:user_id` 和用户列表接口的公开路径、响应字段和错误语义 MUST 与迁移前保持兼容
- **Then** 用户不存在和内部错误仍 MUST 由 Service 将领域或基础设施错误映射为统一响应错误

#### Scenario: Validation boundary refactor keeps auth APIs compatible
- **Given** 登录、改密或刷新请求校验逻辑从 Service 迁移到 Controller 或服务内 validators 层
- **When** 调用方提交合法或非法认证会话请求
- **Then** 登录、改密、刷新、退出当前设备和退出全部设备接口的公开路径、请求字段、响应字段和错误语义 MUST 与迁移前保持兼容
- **Then** 凭据无效、token 无效、session 缺失、用户不存在和内部错误仍 MUST 由 Auth Service 按认证会话语义映射为统一响应错误
