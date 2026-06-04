## ADDED Requirements

### Requirement: Separate request validation from service business logic
用户服务 MUST 将请求级输入清洗和基础参数校验与 Service 业务编排分离。Controller MUST 负责 HTTP 参数提取和响应输出；服务内 Validation 层 MUST 负责请求字段清洗、格式校验和不依赖持久化状态的跨字段校验；Service MUST 负责业务编排、领域规则和 DTO 映射；Repository MUST 负责 Ent/PostgreSQL 数据访问。

#### Scenario: Normalize request input before service call
- **Given** 调用方提交包含前后空白的创建用户、列表用户、用户 ID、登录、改密或刷新请求参数
- **When** controller 调用用户服务业务方法前处理请求
- **Then** 请求级字符串裁剪和基础格式校验 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Service MUST 接收已经规范化并通过基础校验的输入

#### Scenario: Keep business checks in service
- **Given** 创建用户、登录、改密或刷新请求已经通过请求级校验
- **When** Service 编排用户资料或认证会话流程
- **Then** Service MUST 负责用户名唯一性检查、凭据认证、密码哈希、用户 ID 生成、默认业务状态、JWT claims 校验、session/token version 校验和领域错误映射
- **Then** Validation 层 MUST NOT 直接访问 Repository 或 Ent client 执行持久化状态检查

#### Scenario: Keep database access in repository
- **Given** 用户服务需要查询、创建或检查用户记录
- **When** Service 执行业务流程
- **Then** Service MUST 通过 `repository.UserRepository` 访问数据
- **Then** Controller 和 Validation 层 MUST NOT 直接调用 Ent client 或 PostgreSQL 实现包

### Requirement: Provide service-local validation for user request rules
用户服务 MUST 在服务内保留可维护的请求校验边界，用于承载用户服务特定的清洗、基础校验和复杂请求规则，并 MUST 复用 `common/validation` 和 `common/response` 的共享能力。用户服务特定规则 MUST NOT 上移到 `common`，除非多个服务存在稳定复用需求。

#### Scenario: Validate complex user request rules locally
- **Given** 用户服务接口需要校验用户名、昵称、密码、状态、分页、过滤条件或请求体 token 字段
- **When** 这些规则属于用户服务业务上下文但不需要数据库状态
- **Then** 规则 MUST 位于 `user-services/internal/validation` 或等价服务内请求校验边界
- **Then** Controller MUST 调用该校验边界而不是内联大量复杂字段规则

#### Scenario: Reuse shared validation primitives
- **Given** 用户请求需要绑定 Gin URI、query、JSON 或 form 参数并执行结构体校验
- **When** Controller 处理请求
- **Then** Controller MUST 优先复用 `common/validation` 的共享绑定、结构体校验、字段明细和错误归一化能力
- **Then** 服务内 Validation 层 MUST NOT 重复实现响应信封、中间件或基础设施能力

### Requirement: Preserve public API behavior during validation boundary changes
用户服务调整请求校验职责边界时 MUST 保持现有 HTTP 路由、请求字段、响应信封、公开错误语义和用户资料响应字段兼容。

#### Scenario: Validation boundary refactor keeps create API compatible
- **Given** 创建用户请求校验逻辑从 Service 迁移到 Controller 或服务内 Validation 层
- **When** 调用方提交合法或非法创建用户请求
- **Then** `POST /api/v1/users` 的路径、请求字段、成功响应字段和失败响应信封 MUST 与迁移前保持兼容
- **Then** 校验失败仍 MUST 通过 `common/response.Envelope` 输出统一 HTTP 400 响应

#### Scenario: Validation boundary refactor keeps query API compatible
- **Given** 查询用户请求校验逻辑从 Service 迁移到 Controller 或服务内 Validation 层
- **When** 调用方提交合法或非法查询请求
- **Then** `GET /api/v1/users/:user_id` 和用户列表接口的公开路径、响应字段和错误语义 MUST 与迁移前保持兼容
- **Then** 用户不存在和内部错误仍 MUST 由 Service 将领域或基础设施错误映射为统一响应错误

#### Scenario: Validation boundary refactor keeps auth APIs compatible
- **Given** 登录、改密或刷新请求校验逻辑从 Service 迁移到 Controller 或服务内 Validation 层
- **When** 调用方提交合法或非法认证会话请求
- **Then** 登录、改密、刷新、退出当前设备和退出全部设备接口的公开路径、请求字段、响应字段和错误语义 MUST 与迁移前保持兼容
- **Then** 凭据无效、token 无效、session 缺失、用户不存在和内部错误仍 MUST 由 Auth Service 按认证会话语义映射为统一响应错误
