## Context

`user-services/internal` 当前已经具备共享运行时、路由、DTO、领域模型、校验器、service 和 repository 边界，但仍主要按技术层组织核心业务代码：`controller/`、`service/`、`repository/` 同时承载用户资料、用户列表和认证会话能力。随着认证、创建用户、查询用户、列表用户和后续用户能力继续扩展，这种结构会让端口、DTO、领域模型、store 实现和跨能力适配关系分散在多个全局包中。

本变更将用户服务重构为能力本地聚合结构：`internal/user` 聚合用户能力，`internal/auth` 聚合认证能力；`internal/bootstrap`、`internal/router`、`internal/validators` 继续作为服务运行时、路由挂载和纯校验边界。`common/` 仍只承载跨服务稳定能力，服务特定规则不因“未来可能复用”上移到 common。

该重构必须保持外部行为不变：HTTP 路径、请求/响应 JSON 字段、响应信封、错误码、配置 key、Redis key、数据库 schema、migration 历史、Ent 生成代码和 Go module path 都不应变化。

## Goals / Non-Goals

**Goals:**

- 将用户和认证业务代码迁移到 `user-services/internal/user` 与 `user-services/internal/auth` 能力目录。
- 将 HTTP DTO 放入对应能力的 `api/` 子包，将业务 command/query 放入能力根包的 `commands.go`。
- 将 service 消费侧端口放入能力根包的 `ports.go`，接口只暴露当前 use case 必需方法。
- 将 Ent/PostgreSQL 用户 store 放入 `internal/user/store/postgres`，将 Redis 认证会话 store 放入 `internal/auth/store/redis`。
- 将 Ent predicate 构建保留在 PostgreSQL store 内部，禁止 service 层 import Ent predicate 或 Ent user query helper。
- 更新 `AGENTS.md`，把 `internal/shared`、ports、validators、adapter、request DTO/command 映射、predicate 封装规则写成可执行规范。
- 更新相关 OpenSpec 主规格的 delta，确保归档后规范能成为长期约束。

**Non-Goals:**

- 不新增或修改 HTTP API。
- 不修改统一响应信封、错误码或公开错误文案。
- 不修改配置结构、环境变量覆盖规则、Redis key 策略或日志字段。
- 不修改 Ent schema、Atlas migration 或数据库表结构。
- 不引入新的外部依赖。
- 不手写 `user-services/ent/` 下的 Ent 生成代码。

## Decisions

### Decision: 按能力聚合用户服务代码

将 `controller/user_controller.go`、`service/user_service.go`、用户 DTO、用户领域模型、用户错误、用户命令、用户 adapter 和 PostgreSQL store 聚合到 `internal/user`；将 `controller/auth_controller.go`、认证 DTO、认证 service、认证 token/session 组件和 Redis store 聚合到 `internal/auth`。

理由：能力目录能让 controller、service、ports、model、adapter 和 store 的依赖关系在同一边界内可见，减少全局 `service` 或 `repository` 包承担多种能力契约的倾向。

备选方案：继续保持技术分层目录，只补充文档规则。该方案迁移成本低，但无法让包结构本身约束边界，后续更容易出现跨能力耦合和端口污染。

### Decision: ports 属于调用方 service

每个能力根包的 `ports.go` 由本能力 service 的用例需求定义，例如用户创建只暴露创建所需输入和方法，认证凭据验证只暴露认证流程当前需要的读取与更新方法。禁止为了复用 store 实现而把完整 CRUD 接口提升为端口。

理由：六边形架构中端口表达调用方需求，而不是适配器能力清单。端口最小化能避免 service 被基础设施细节或未来假设牵引。

备选方案：由 store 包定义统一 repository 接口，service 直接消费完整接口。该方案短期简单，但会把未使用方法泄漏给 service，并增加测试替身和后续演进成本。

### Decision: request DTO 与 command/query 分离

`api/request.go` 只承载 HTTP 协议适配字段和 tag；`commands.go` 承载不依赖 Gin、HTTP tag 或 binding 语义的应用层输入。`controller.go` 负责把 request DTO 映射为 command/query，并可在映射时补充 `ClientIP`、`UserAgent`、`TraceID`、`OperatorID` 等 HTTP 上下文信息。

理由：service 不应被传输层 DTO 绑住。分离后，同一用例可被 HTTP、CLI、测试或后续内部调用复用。

备选方案：service 继续直接接收 request DTO。该方案改动少，但会让 Gin/HTTP 语义进入业务编排层。

### Decision: validators 只保留纯业务校验

`internal/validators` 保留为服务内纯函数校验集合，只处理用户名格式、密码强度、输入规范化、分页边界等无状态规则。用户名是否存在、邮箱是否注册、session 是否有效等依赖 DB、Redis 或外部系统的规则必须在 service 中通过 port 编排完成。

理由：validators 保持无外部依赖后可以简单单测、复用和审查，不会成为隐藏的数据访问层。

备选方案：把所有校验都放入 validators。该方案表面统一，但会让 validators 依赖 repository 或 Redis，破坏分层。

### Decision: adapter.go 只做跨能力极简适配

能力根包允许提供 `adapter.go`，用于向其他能力暴露极简读取或协作接口。adapter 可做字段裁剪、结果映射和轻量封装，但复杂业务编排、事务、策略决策和多依赖协调必须留在 service 中。

理由：adapter 的价值是防止调用方直接依赖本能力内部模型或 store；如果 adapter 承担业务编排，就会形成第二个 service 层并扩大反向耦合。

备选方案：禁止 adapter.go。该方案边界更硬，但会迫使其他能力直接依赖 service 的完整接口或领域模型，不利于最小暴露。

### Decision: Ent predicate 封装在 store 内部

用户查询条件拼装由 `internal/user/store/postgres/predicates.go` 负责，service 只传递业务语义 query/input，不能 import `user-services/ent/user` 或 `user-services/ent/predicate`，也不能调用 `user.StatusEQ` 等 ORM predicate helper。

理由：Ent 是 PostgreSQL adapter 的实现细节。将 predicate 留在 store 内部可保持 service 与 ORM 解耦，也让未来替换查询实现或增加索引策略时不影响业务层。

备选方案：service 直接构造 predicate 再传给 repository。该方案灵活，但会让 ORM 细节外泄并破坏端口抽象。

### Decision: internal/shared 默认不创建

不主动创建 `internal/shared/`。只有无法通过 ports 或依赖注入解决、多个能力必须稳定共享、且属于原子级 Value Object 或极少量跨能力错误定义时，才允许新增该目录内容；新增时必须在说明中回答为什么不能用 port/DI、为什么稳定跨能力共享、为什么不是业务能力下沉。

理由：`shared` 目录最容易退化为无边界 helper 集合。默认禁止比事后清理更可靠。

备选方案：创建 `internal/shared` 承载通用 helper。该方案短期方便，但会持续削弱能力目录边界。

## Risks / Trade-offs

- 包路径迁移影响面较大 → 先完成机械迁移和 import rewiring，再逐步收紧端口/command 边界，并用现有测试验证行为。
- Fx provider wiring 可能因接口或构造函数迁移失败 → 在 `bootstrap` 中集中调整 provider annotation，并运行 user-services 全量测试。
- Swagger 注释或生成引用可能仍指向旧 DTO 包 → 迁移 DTO 后检查 controller 注释、`docs/` 生成物和 swagger 测试。
- 循环依赖风险上升 → 能力根包定义 service、ports、commands、model；store 子包只能向内依赖能力根包和 Ent，不允许能力根包依赖 store 子包。
- 文档规则可能与现有代码短暂不一致 → `/opsx:apply` 应把代码迁移、AGENTS.md 和 specs 一起完成，避免规范先行但代码不落地。

## Migration Plan

1. 移动用户能力代码到 `internal/user`，拆分 `api/`、`store/postgres/`、`commands.go`、`ports.go`、`model.go`、`errors.go`、`mapper.go` 和可选 `adapter.go`。
2. 移动认证能力代码到 `internal/auth`，拆分 `api/`、`store/redis/`、`commands.go`、`ports.go`、`model.go`、`rediskeys.go` 和 service 组件。
3. 更新 `bootstrap` 和 `router` 的 import、provider annotation 和 controller 类型引用。
4. 将 request DTO 到 command/query 的映射放入 controller，移除 service 对 HTTP request DTO 的直接依赖。
5. 将 Ent predicate 构建拆到 `internal/user/store/postgres/predicates.go`，确保 service 不 import Ent 查询构造包。
6. 更新 `AGENTS.md` 和 capability map。
7. 运行 `gofmt`，分别在 `common/` 和 `user-services/` 运行 `go test ./...`。

Rollback 策略：该变更不改变外部协议或数据结构；如迁移过程中出现不可控问题，可按包迁移粒度回退对应文件移动和 import rewiring，不需要数据库或配置回滚。

## Open Questions

- 无阻塞问题。实现时若发现某个跨能力类型无法通过 port 或依赖注入表达，必须先按 `internal/shared` 审查条件补充说明，再决定是否创建 shared 内容。
