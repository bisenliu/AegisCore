## ADDED Requirements

### Requirement: Restrict service-local shared directory usage

系统 SHALL 将 `user-services/internal/shared` 作为默认不创建的例外目录管理。只有无法通过 ports 或依赖注入解决，且多个服务内能力必须稳定共享的原子级 Value Object 或极少量跨能力错误定义，才允许进入 `internal/shared`。业务逻辑、工具函数、流程编排、store、service、controller 和 DTO MUST NOT 放入 `internal/shared`。

#### Scenario: Reject shared directory for business helpers
- **Given** 开发者准备新增用户服务内部通用 helper、业务流程、DTO、controller、service 或 store 代码
- **When** 该代码只服务于单一能力，或可以通过能力本地 package、ports 或依赖注入表达协作
- **Then** 实现 MUST NOT 创建或使用 `user-services/internal/shared`
- **Then** 代码 MUST 保持在所属能力目录或明确的运行时边界内

#### Scenario: Allow atomic shared value object after review
- **Given** 多个能力必须共享同一个稳定的原子级 value object，例如服务内跨能力统一的用户 ID 类型
- **When** 该类型无法通过 ports 或依赖注入避免直接共享
- **Then** 开发者 MAY 在 `user-services/internal/shared` 下新增最小内容
- **Then** 变更说明 MUST 解释为什么不能通过 ports 或依赖注入解决、为什么属于多个能力稳定共享、为什么是原子级基础语义而非业务能力下沉

#### Scenario: Preserve common and service-local boundaries
- **Given** 开发者准备新增共享代码
- **When** 该代码属于跨服务稳定契约、运行时基础能力、HTTP 适配、安全凭证原语或通用校验核心
- **Then** 代码 MUST 进入 `common` 对应能力目录
- **Then** 只对用户服务有效的规则 MUST 留在 `user-services` 对应能力边界内
