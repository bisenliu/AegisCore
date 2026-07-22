## Context

当前 RBAC CLI 提供 `rbac seed`、`rbac assign-super-admin` 和 `rbac create-super-admin`。其中 `create-super-admin` 可创建或复用用户、可通过 flag 或环境变量重置密码，`assign-super-admin` 可离线给任意用户绑定超级管理员角色。这些能力适合早期初始化，但长期运维中会扩大离线高权限操作面，并且与在线 RBAC 用户角色绑定、认证强制改密和 policy sync 边界重叠。

本变更只面向全新数据库初始化。系统在 `rbac seed` 后执行一次 `rbac bootstrap-super-admin`，创建代码固定 UUID 的 bootstrap 用户并绑定代码固定的超级管理员角色。是否已引导不通过新表记录，而是通过固定用户 ID 是否存在判断；即使该用户软删除、禁用、角色绑定丢失或 username 被修改，也视为已完成引导。

受影响路径主要包括 `user-service/cmd/rbac.go`、`user-service/internal/features/role/application/bootstrap/`、`user-service/internal/features/role/infrastructure/postgres/`、role provider/wiring、Makefile、RBAC/auth/delivery OpenSpec 规格和测试。`common` 不新增能力；`internal/shared` 仅继续消费 `identity` 和 `rbacbaseline`；不新增 `internal/integration`、MQ、eventbus、outbox、Ent schema 或 Atlas migration。

## Goals / Non-Goals

**Goals:**

- 将首次超级管理员初始化替换为一次性的 `rbac bootstrap-super-admin`。
- 使用代码固定的 bootstrap 用户 UUID 和现有 `rbacbaseline.SuperAdminRoleID`，禁止 CLI 覆盖用户 ID 或角色 ID。
- 在 role application 中提供 framework-neutral bootstrap service，负责输入校验、归一化、密码策略校验、密码哈希和调用最小 `BootstrapStore` port。
- 在 role PostgreSQL infrastructure 中用同一事务和固定 transaction advisory lock 完成角色校验、固定用户 ID 占用检查、username 占用检查、用户创建和角色绑定。
- 保证重复执行、唯一约束冲突和前置条件失败映射为稳定应用错误，不暴露 Ent 或 PostgreSQL 原始错误。
- 让 bootstrap 用户以 `identity.UserStatusMustChangePassword` 创建，并复用现有强制改密认证流程完成凭据更新与撤销语义。
- 删除旧 CLI、旧 Makefile 目标和旧环境变量契约，明确全新数据库的发布顺序。

**Non-Goals:**

- 不支持旧数据库原地升级、旧超级管理员识别、marker 回填或旧数据迁移。
- 不保留 `create-super-admin`、`assign-super-admin`、旧 flags、旧环境变量或别名兼容。
- 不新增 `rbac_bootstrap_states` 表、数据库 trigger、Ent schema 或 Atlas migration。
- 不提供 `recover-super-admin`、离线密码重置、force、reuse、reactivate 或 user ID 参数。
- 不在 bootstrap CLI 中实现强制改密后的 token version、Redis 投影、本地缓存或 refresh session 撤销逻辑。
- 不改变在线用户角色绑定 API 的授权、policy version 发布或缓存收敛设计。

## Decisions

### 固定用户 ID 作为引导完成标识

使用 `BootstrapSuperAdminUserID = "00000000-0000-0000-0000-000000000002"` 判断是否已完成引导。查询必须包含软删除用户，不添加 `deleted_at IS NULL`，只要该 ID 存在就返回 `ErrSuperAdminAlreadyBootstrapped`。

备选方案是新增 `rbac_bootstrap_states` 表记录引导状态。该方案会引入额外 schema、migration 和一致性问题，并允许 marker 与实际用户状态不一致；固定用户 ID 更简单，也符合全新数据库一次性引导的约束。

### 应用层只处理业务输入和最小 port

新增 `user-service/internal/features/role/application/bootstrap/`，建议包含 `service.go`、`ports.go`、`errors.go` 和 `service_test.go`。service 校验 `--username`、`--nickname` 和密码环境变量内容，完成 username trim + lowercase、nickname fallback、密码字节长度 12 至 72 校验和 bcrypt hash，然后调用 `BootstrapStore.BootstrapSuperAdmin(ctx, input)`。

备选方案是把校验和哈希放入 Cobra command 或 PostgreSQL adapter。该方案会把业务规则分散到 transport/infrastructure，降低单元测试覆盖质量，并违反 application/domain 不依赖 CLI 或 Ent 细节的边界。

### PostgreSQL adapter 拥有事务和并发控制

新增 `user-service/internal/features/role/infrastructure/postgres/bootstrap_store.go`。一次 bootstrap 在同一 PostgreSQL 事务内执行固定 transaction advisory lock、角色校验、固定用户 ID 查询、username 全量占用检查、用户创建和用户角色绑定。任何失败必须回滚，不能留下没有角色的用户、状态错误的用户或孤立绑定。

备选方案是依赖唯一约束和普通事务，不使用 advisory lock。该方案仍可避免部分重复写入，但并发失败路径更依赖底层唯一约束错误，难以稳定表达“最多一个成功”的业务语义；固定 advisory lock 让并发行为更明确。

### 只允许全新数据库部署路径

发布顺序固定为 `Atlas migration -> rbac seed -> bootstrap-super-admin -> 启动所有 HTTP 副本 -> 初始管理员强制改密`。旧库升级、旧管理员数据识别、自动恢复和双版本 CLI 共存均不支持。

备选方案是提供旧命令兼容和旧数据迁移。该方案会保留高权限离线入口，增加状态判断分支，并与“不兼容旧命令和旧数据”的安全目标冲突。

### 后续授权只走在线 RBAC

删除离线 `assign-super-admin` 后，后续超级管理员授权只能通过现有在线用户角色绑定 API。在线流程继续负责权限校验、policy version 发布、Pub/Sub 通知、本地 reload 或缓存失效。

备选方案是保留离线绑定作为应急入口。该方案会绕过在线授权和 policy sync 契约，继续扩大离线高权限操作面；所有管理员不可用时，本方案明确只允许 DBA 人工介入或重新初始化数据库。

### 不改变认证强制改密实现

bootstrap CLI 只创建 `identity.UserStatusMustChangePassword` 用户。首次登录、password-change token、条件更新、token version 更新、Redis 投影刷新、本地缓存失效和 refresh session 撤销继续由 auth feature 的现有强制改密能力完成。

备选方案是在 CLI 创建后直接设置正常状态或调用认证撤销逻辑。前者降低临时密码安全性；后者把 auth runtime 撤销职责引入离线 role CLI，扩大耦合并增加部分失败风险。

## Risks / Trade-offs

- 固定用户 ID 被硬删除后可能允许再次引导 → 当前系统没有用户硬删除能力；未来新增硬删除时必须禁止硬删除固定 bootstrap 用户 ID。
- 所有超级管理员不可用时没有应用内恢复入口 → 明确运维策略为 DBA 人工介入或重新初始化数据库，避免保留高权限离线恢复面。
- 删除旧命令会破坏现有脚本 → 这是有意的 breaking change；Makefile、文档、help 和测试必须同步只展示新命令。
- bootstrap 在 HTTP 副本运行期间执行不会触发在线 policy refresh → 发布顺序要求在启动副本前完成；如果违反顺序，运维必须滚动重启或显式触发在线收敛。
- 密码不 trim 可能让运维误把首尾空格写入密码 → 该行为是安全契约；命令不得打印密码，测试必须覆盖首尾空格保留。
- PostgreSQL advisory lock key 需要稳定且避免冲突 → adapter 使用 feature-local 固定常量并只用于该 bootstrap 事务，不下沉到 common。

## Migration Plan

1. 删除旧 RBAC CLI 命令、flags、环境变量读取和 Makefile 目标。
2. 新增 role bootstrap application service、错误、port 和单元测试。
3. 新增 role PostgreSQL bootstrap store，覆盖事务、软删除包含查询、唯一约束映射、advisory lock 和并发执行。
4. 更新 wiring，使 `rbac bootstrap-super-admin` 使用正式 config、Ent/PostgreSQL 资源、bootstrap service 和 store。
5. 更新 Makefile、help、开发/测试说明和部署顺序文档中旧命令示例。
6. 增加 seed -> bootstrap -> 强制改密 -> 正常登录 -> 超级管理员授权 E2E。
7. 验证 `make user-service-architecture-lint`、相关 Go package 测试、E2E、`make lint` 和 `make verify`。

回滚策略是代码回滚到包含旧命令的版本。由于本变更不新增 schema 和 migration，数据库结构无需回滚；但已经创建的固定 bootstrap 用户会作为普通业务数据保留。该 change 明确不支持新旧 CLI 双版本共存，因此发布必须以全新数据库和单版本 CLI 为前提。

## Open Questions

- 无。
