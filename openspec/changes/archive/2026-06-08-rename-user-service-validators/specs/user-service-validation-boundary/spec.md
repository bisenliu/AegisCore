## MODIFIED Requirements

### Requirement: Provide service-local validation for user request rules
用户服务 MUST 在服务内保留可维护的校验器集合，用于承载用户服务特定的清洗、基础校验、解析、转换和复杂请求规则，并 MUST 复用 `common/validation` 和 `common/contract/response` 的共享能力。用户服务特定规则 MUST NOT 上移到 `common`，除非多个服务存在稳定复用需求。服务内校验器集合 MUST 位于 `user-services/internal/validators`，并可按领域拆分为 `user.go`、`auth.go`、`team.go` 等文件。

#### Scenario: Validate complex user request rules locally
- **Given** 用户服务接口需要校验用户名、昵称、密码、状态、分页、过滤条件或请求体 token 字段
- **When** 这些规则属于用户服务业务上下文但不需要数据库状态
- **Then** 规则 MUST 位于 `user-services/internal/validators` 或等价服务内校验器边界
- **Then** Controller MUST 调用该校验器边界而不是内联大量复杂字段规则

#### Scenario: Reuse shared validation primitives
- **Given** 用户请求需要绑定 Gin URI、query、JSON 或 form 参数并执行结构体校验
- **When** Controller 处理请求
- **Then** Controller MUST 优先复用 `common/validation` 的共享绑定、结构体校验、字段明细和错误归一化能力
- **Then** 服务内校验器层 MUST NOT 重复实现响应信封、中间件或基础设施能力

#### Scenario: Keep validators free of persistence checks
- **Given** 用户服务需要检查用户名唯一性、认证凭据、会话状态、权限或其他依赖持久化状态的业务规则
- **When** Service 编排用户资料、认证或团队相关流程
- **Then** `user-services/internal/validators` MUST NOT 直接访问 Repository、Ent client、Redis client 或外部服务执行该检查
- **Then** 该检查 MUST 保持在 Service 或其明确依赖的领域编排边界中

#### Scenario: Organize validators by domain file
- **Given** 用户服务存在 user、auth、team 或 toll team 等不同领域的输入规则
- **When** 这些规则位于服务内校验器集合中
- **Then** 实现 MUST 按领域拆分为 `user.go`、`auth.go`、`team.go` 等文件
- **Then** 包名 MUST 使用 `validators`，允许包含 `Normalize`、`Validate`、`Parse` 等函数
