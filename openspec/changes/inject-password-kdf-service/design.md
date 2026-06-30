## Context

`common/security/password` 当前以包级 `argon2Gate` 和 `argon2Queue` 限制同一进程内 Argon2id 执行并发和等待队列。该实现不会在不同 OS 进程之间共享 channel，但会把密码 KDF 的资源预算固定在共享包内部，导致 user-service、未来服务、CLI 和测试无法按各自资源模型独立声明预算。

本次 change 是破坏式 API 变更：不保留包级 `HashContext`、`VerifyContext` 兼容入口。调用方必须显式持有密码 KDF 服务实例，并通过实例方法进行哈希与校验。

受影响路径：

- `common/security/password/`：新增可实例化服务、配置结构和实例级门控；删除包级 KDF 函数和包级门控变量。
- `user-service/internal/features/auth/application/credentials/`：登录校验和强制改密改为依赖注入的密码服务。
- `user-service/internal/features/user/application/command/`：创建用户改为依赖注入的密码服务。
- `user-service/cmd/rbac.go`：超级管理员创建或重置密码时显式构造或接收密码服务实例。
- `user-service/internal/providers/` 与配置结构：在服务 provider 边界创建密码服务实例，并从配置读取资源预算。
- 测试：所有直接调用 `password.HashContext` 和 `password.VerifyContext` 的测试改为创建测试专用实例。

不影响 HTTP API、数据库 schema、OpenAPI 生成物、Redis key、Casbin policy sync、部署清单和观测资产。

## Goals / Non-Goals

**Goals:**

- 让密码 KDF 资源预算成为调用方显式配置，而不是 `common` 包级默认状态。
- 保持 Argon2id 算法、编码哈希格式、明文密码长度限制、哈希解析和常量时间比较语义不变。
- 允许同一进程内多个服务组件或测试使用不同的密码 KDF 实例，实例之间不共享队列或执行槽位。
- 让 user-service 的认证、用户创建和 RBAC CLI 全部迁移到实例化密码服务。

**Non-Goals:**

- 不更换密码算法，不调整 Argon2id 内存、迭代、并行度、盐长度或 key 长度策略。
- 不引入跨进程或跨节点的 KDF 分布式门控。
- 不新增 HTTP API、数据库字段、migration、OpenAPI 注解或部署资源。
- 不在 `user-service/internal/shared`、feature HTTP transport、Redis adapter 或外部 integration 包中放置密码 KDF 构造逻辑。

## Decisions

### Decision: 在 `common/security/password` 提供实例化服务

`common/security/password` 继续拥有业务中立的 Argon2id primitive，包括参数校验、哈希编码、哈希解析、KDF 执行和常量时间比较。新的公开入口为类似 `Service` 与 `Options` 的实例化 API，实例内部持有 `gate` 和 `queue` channel。

备选方案：保留包级默认实例并额外提供可配置实例。该方案迁移成本较低，但会继续允许新代码绕过显式配置，和“不需要兼容”的目标冲突，因此不采用。

### Decision: 密码算法参数保持固定，资源预算可配置

哈希格式中的 Argon2id 安全参数继续由 `common/security/password` 统一维护，调用方只配置并发上限和队列上限。这样避免不同服务生成互不兼容或安全等级漂移的密码哈希，同时解决资源预算不可配置的问题。

备选方案：把 Argon2id 内存、迭代、并行度也开放给服务配置。该方案会扩大密码兼容性和升级策略复杂度，本次 change 不需要，因此不采用。

### Decision: user-service 在 provider/config 边界创建实例

HTTP 服务运行时应在 `user-service/internal/providers/` 读取服务配置并提供密码 KDF 实例，再注入 auth/user application。CLI 路径不经过完整 HTTP Fx app 时，应在 CLI 命令装配边界显式创建实例，避免把构造逻辑塞入 feature application 或 infrastructure adapter。

备选方案：在每个 use case 内直接调用 `password.NewService`。这会把资源预算装配分散到业务层，降低可测试性，也违背 provider 边界职责，因此不采用。

### Decision: 测试使用独立实例，不修改包级状态

测试应通过小容量实例验证队列满、等待取消和实例隔离等行为。删除包级 channel 后，测试不再需要在同包内替换全局变量，也不会因测试顺序污染后续用例。

备选方案：保留仅测试可见的 reset hook。该方案仍然依赖全局状态，不采用。

## Risks / Trade-offs

- [Risk] 破坏式 API 会导致所有调用点必须一次性迁移 → Mitigation：通过 `rg "password\\.(HashContext|VerifyContext)"` 列出调用点，迁移后用同一命令确认零残留。
- [Risk] 新配置缺失会导致 user-service 启动失败 → Mitigation：为配置结构提供明确字段和 validation，默认配置文件同步添加生产可用默认值。
- [Risk] CLI 与 HTTP runtime 使用不同装配路径，可能遗漏密码服务构造 → Mitigation：把 RBAC CLI 作为独立任务和测试/构建目标覆盖。
- [Risk] 实例化后测试仍使用生产级 Argon2id 参数，执行时间较长 → Mitigation：本次只开放资源预算，不降低算法参数；测试可控制并发/队列，但不绕过 KDF 安全参数。
- [Risk] 下游服务尚未迁移会无法编译 → Mitigation：本仓库内所有 Go package 必须通过 `make test`、`make lint` 和 `make verify` 后才完成 change。

## Migration Plan

1. 在 `common/security/password` 引入实例化服务，删除包级 KDF 函数和全局门控。
2. 更新 common password 单元测试，覆盖实例构造、无效预算、队列满、等待取消和实例隔离。
3. 扩展 user-service 配置和 provider，创建密码 KDF 服务实例并注入 auth/user application。
4. 迁移 auth、user 和 RBAC CLI 调用点，删除所有包级 `password.HashContext` / `password.VerifyContext` 调用。
5. 运行相关 package 测试、`make lint` 和 `make verify`。

回滚方式：在实现提交前可整体回退本 change 的代码改动；合并后如需回滚，必须同时恢复包级 API、调用方迁移和配置变更，因为本次不保留兼容入口。

## Open Questions

无。已确认本次不保留兼容入口，且只配置实例级并发和队列预算。
