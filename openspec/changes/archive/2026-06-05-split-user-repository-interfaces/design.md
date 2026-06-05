## Context

`user-services/internal/repository/user_repository.go` 当前定义的 `UserRepository` 同时包含 `Create`、`GetByUsername`、`GetByUserID`、`GetTokenVersion`、`IncrementTokenVersion`、`UpdateCredentials` 和 `ListUsers`。这些方法跨越用户资料创建、用户资料查询、认证凭证读取与更新、用户级 token version 读取与递增、用户列表等多个能力边界。

`user-services/internal/service/user_service.go` 的 `userService` 实际只需要创建用户、按用户 ID 查询和列表查询。`user-services/internal/service/auth_service.go` 中的认证编排则把同一个大接口传给凭证验证和会话管理组件，但凭证组件主要需要按用户名读取用户、按用户 ID 读取用户状态并更新凭证；会话组件主要需要读取和递增 token version。不同消费方共同依赖一个大接口，体现为典型的接口隔离原则不足。

这类设计的直接影响是测试替身需要实现大量无关方法，服务和认证能力在仓储抽象层产生不必要耦合，并且某一类仓储能力调整时容易引发跨 capability 编译或测试变更。本变更只调整内部 Go 抽象和依赖注入边界，不改变 controller/service/repository 分层职责，不改变 HTTP API、响应信封、错误映射、Ent schema、PostgreSQL 表结构或 Redis 会话语义。

## Goals / Non-Goals

**Goals:**

- 将用户仓储抽象按消费端能力拆分为三个小接口，分别服务用户资料、认证凭证和 token version 场景。
- 让 `userService`、认证凭证组件和认证会话组件只声明最小必需依赖。
- 保持现有 PostgreSQL repository 结构体和方法集合不拆散，通过 Go 隐式接口实现完成低成本适配。
- 调整 Fx 装配，使同一个底层 PostgreSQL 用户仓储实例可以作为多个小接口注入。
- 简化单元测试 fake/mock，减少无关方法空实现。

**Non-Goals:**

- 不新增用户 API 或认证 API。
- 不调整响应 envelope、业务错误码或公开错误消息。
- 不修改 Ent schema、生成代码、Atlas migration 或数据库结构。
- 不拆分 `repository/postgres` 中现有用户仓储结构体，也不强制把每组方法移动到不同实现类型。
- 不把服务特定仓储接口上移到 `common`。

## Decisions

### Decision 1: 按消费端能力拆分为三个小接口

拆分后的仓储能力建议为：

- `UserProfileRepository`：包含用户资料创建、按外部用户 ID 查询、用户列表查询；消费方为 `userService`。
- `UserCredentialRepository`：包含认证凭证读取和凭证更新所需方法；消费方为认证凭证组件，例如 `CredentialVerifier` 或后续承担改密持久化的凭证组件。
- `UserTokenVersionRepository`：包含 token version 读取和原子递增；消费方为认证会话组件，例如 `AuthSessionManager`，以及需要回源读取 token version 的会话存储路径。

选择该方案的原因是接口边界与实际调用方一致，能直接降低测试替身复杂度，并减少用户资料 capability 与认证 capability 在仓储接口上的互相污染。备选方案是保留 `UserRepository` 并在测试中使用嵌入或通用 mock 生成器，但这只能缓解测试样板，无法解决调用方依赖过宽和 capability 边界不清的问题。

### Decision 2: 保留底层 PostgreSQL 实现结构不变

`repository/postgres` 中现有用户仓储结构体已经具备这些方法，Go 的隐式接口实现允许同一个结构体同时满足多个小接口。实现层不需要按接口拆成多个结构体，也不需要迁移已有方法，只需要在编译期确保该结构体的方法集合覆盖三个接口。

选择该方案的原因是它最小化变更范围，避免把纯抽象层重构扩大为数据访问实现重组。备选方案是将 PostgreSQL 实现拆为 profile、credential、token version 三个结构体，但当前没有独立连接、事务、生命周期或存储后端差异，拆散结构体会增加装配和维护成本。

### Decision 3: Fx 以同一实例的多个接口身份注入

Fx 装配应先构造一个底层 PostgreSQL 用户仓储实例，再通过 provider 或 `fx.Annotate`/`fx.As` 等方式将该实例暴露为三个接口。关键约束是复用同一个底座对象，而不是为三个接口重复创建三个独立 repository 实例。

选择该方案的原因是它保持底层资源生命周期清晰，并避免未来 repository 内部持有缓存、metrics 或事务辅助状态时出现多实例语义差异。备选方案是每个接口各自提供一个构造函数并重复创建实现对象，但这会让依赖图更分散，也可能隐藏实例重复问题。

### Decision 4: 消费方依赖收敛到最小接口

`userService` 构造函数应只接收用户资料仓储接口。认证服务的 Fx 参数不应继续暴露完整用户仓储大接口，而应分别向凭证组件传入凭证仓储接口、向会话组件传入 token version 仓储接口。认证服务自身继续承担用例编排，但不保存或直接使用完整用户 repository。

选择该方案的原因是服务字段和构造函数本身就是架构边界的可读契约。依赖声明越窄，越能防止后续在错误层级调用不该调用的仓储能力。备选方案是在 `NewAuthService` 中仍接收大接口并向内部组件手动转交子能力，但这会把接口隔离停留在内部实现层，Fx 和测试边界仍然依赖过宽。

## Risks / Trade-offs

- [接口拆得过细] → 以消费端稳定职责为边界，不按每个方法拆接口；本次保持三个接口，避免 `CreateUserRepository`、`GetUserRepository` 等过度碎片化命名。
- [方法归属存在争议] → `GetByUserID` 在用户资料和改密状态校验中都可能使用。若凭证组件需要按 ID 读取用户状态，可以允许 `UserCredentialRepository` 也包含按 ID 读取认证相关用户实体，或进一步定义语义更明确的认证资料读取方法；不应为了去重强迫凭证组件依赖资料接口。
- [旧规格中仍引用 `repository.UserRepository`] → 归档时需要更新相关主规格措辞，把“根 `repository.UserRepository` 抽象”改为按消费端拆分后的接口名，避免长期规格与实现不一致。
- [Fx 多接口注入可能产生重复实例] → 装配时明确先创建一个实现实例，再以多个接口身份导出；测试或审查中检查 provider 不重复打开数据库连接或重复创建 repository。
- [实现层仍有大结构体] → 这是有意选择。接口隔离解决的是消费方依赖面，不要求持久化实现按同样维度物理拆分；只有当实现内部职责、事务或后端真正分化时再考虑拆结构体。
- [新增能力再次膨胀] → 新增用户相关仓储方法时必须先明确消费方和 capability owner；只有多个消费方稳定共享同一语义时才放入既有接口，否则新增更小的消费端接口或调整对应接口边界。
