## Context

`ReplaceRolePermissions` 当前先通过 `RoleStore.GetByRoleID` 校验角色，再用 `uniqueUUIDs` 对请求中的权限 ID 按首次出现顺序去重，随后逐个调用 role application 的 `PermissionLookup.GetByPermissionID`。该 adapter 又逐次调用 permission application 的 `PermissionStore.GetByPermissionID`，因此 application 预校验会产生与权限数量成正比的 PostgreSQL 查询。

`RolePermissionStore.Replace` 已在写事务内通过 `lockedPermissionsByExternalIDs` 对完整权限集合执行一次批量重校验，再删除旧绑定并批量写入新绑定。这次事务内校验负责封闭 application 预校验与事务写入之间的 TOCTOU 窗口，必须保留。变更跨越 permission 与 role 两个 feature，但仍位于 `user-service` 内部既有消费侧端口和 adapter 边界。

受影响路径包括：

- `user-service/internal/features/permission/application/ports.go`
- `user-service/internal/features/permission/infrastructure/postgres/permission_store.go` 及其测试
- `user-service/internal/features/role/application/ports.go`
- `user-service/internal/features/role/application/command/binding.go`、生成的 mocks 及其测试
- `user-service/internal/features/role/infrastructure/postgres/permission_lookup.go` 及其测试
- `user-service/internal/features/role/fx_test.go` 等实现受调整接口的测试替身
- `openspec/changes/optimize-role-permission-bulk-lookup/`

本 change 不需要修改 `common/`、`user-service/internal/shared`、`user-service/internal/integration`、`deployments/` 或主规格之外的 `docs/`。它不改变 RBAC policy sync、安全授权边界或观测日志。

## Goals / Non-Goals

**Goals:**

- 将非空权限集合的 application 预校验收敛为一次 PostgreSQL `WHERE permission_id IN (...)` 查询，使 100 和 1000 个权限 ID 的 permission lookup SQL 查询次数均为 1。
- 规定批量查询的空输入、去重、顺序、缺失和错误语义，并在 permission store、role adapter 与 command 边界保持一致。
- 任一权限缺失或批量查询失败时，在进入 `RolePermissionStore.Replace` 前终止完整替换，不产生绑定写入或 policy change 通知。
- 保留事务内完整集合的批量权限重校验，以及既有原子替换、回滚、policy revision、通知和 reload 行为。
- 保留单权限 `GetByPermissionID`，继续服务 `AddRolePermission` 的合法单权限校验。

**Non-Goals:**

- 不修改用户角色数量上限或角色权限数量上限。
- 不修改 HTTP request DTO、公开 API、错误响应、OpenAPI 注解或 OpenAPI 生成物。
- 不修改 Casbin 授权循环、policy reload、policy revision、outbox 或多副本同步逻辑。
- 不修改数据库 schema、Atlas migration、Ent schema 或 Ent 生成代码。
- 不增加 feature flag、兼容开关、双读、逐条查询回退或生产权限基线数据。
- 不把该服务内 RBAC 查询语义移动到 `common/`、`internal/shared` 或 `internal/integration`。

## Decisions

### Decision: 由 permission application 端口定义批量权限查询

在 `permissionapplication.PermissionStore` 增加 `GetByPermissionIDs(ctx, permissionIDs) ([]permissiondomain.Permission, error)`，同时保留 `GetByPermissionID`。permission feature 继续拥有权限目录查询语义，role feature 只通过其消费的最小端口访问，不直接导入 permission infrastructure 或 Ent predicate。

备选方案是在 role infrastructure 内直接查询 Ent permissions，或让 command 并发调用单条接口。前者破坏 feature 边界，后者仍是 O(N) 次数据库查询且增加连接池压力，因此不采用。

### Decision: permission PostgreSQL store 在一次查询后恢复输入顺序并整体判缺

`GetByPermissionIDs` 先用 map 按首次出现顺序构造去重 ID 列表。空列表直接返回非 nil 空 slice，不调用 Ent 或数据库。非空列表只执行一次 `PermissionIDIn(uniqueIDs...)` 查询；查询结果建立 `permission_id -> Permission` map，再按去重 ID 列表重排。

若重排时任一 ID 不存在，方法返回包装后的 `permissiondomain.ErrPermissionNotFound` 和 nil 结果，不暴露部分成功集合；数据库失败继续使用带操作上下文的 `%w` 包装，保持 `errors.Is` 语义。不能依赖 PostgreSQL `IN` 查询的自然顺序，也不通过每个 ID 的二次查询补齐缺失项。

备选方案是在 SQL 中通过 `ORDER BY array_position` 恢复顺序。该方案把输入顺序编码进数据库表达式，仍需额外处理重复项与缺失项，并使实现和测试更依赖 PostgreSQL 方言；使用 Go map 重排更符合现有 adapter 风格且不增加 SQL 次数。

### Decision: role adapter 批量映射，command 只发起一次批量调用

在 role application 的 `PermissionLookup` 增加 `GetByPermissionIDs(ctx, permissionIDs) ([]PermissionReference, error)`。PostgreSQL adapter 调用 permission store 的同名方法一次，并按返回顺序映射全部字段，不自行排序或逐条回查。

`ReplaceRolePermissions` 继续先校验角色存在，再调用现有 `uniqueUUIDs`，随后只调用一次 `GetByPermissionIDs`，并将返回的有序 `PermissionReference` 集合原样传给 `RolePermissionStore.Replace`。批量 lookup 失败时立即返回，因此 replace、policy change 通知和 reload 均不会执行。`AddRolePermission` 保持调用 `GetByPermissionID`。

备选方案是删除 command 的 `uniqueUUIDs`，完全依赖 store 去重。虽然 store 仍必须独立满足去重契约，但保留 command 现有规范化可以让传入 `Replace` 的集合语义保持明确，也符合本次范围要求。

### Decision: 保留事务内批量权限重校验

`RolePermissionStore.Replace` 的 `lockedPermissionsByExternalIDs` 继续在同一写事务内批量查询并重排权限，再执行旧绑定删除和新绑定批量写入。完整路径因此固定为一次 application 批量查询，加一次事务内批量重校验；后者不属于兼容回退，而是防止权限在两阶段之间被删除所必需的一致性检查。

备选方案是信任 application 查询结果并删除事务内重校验。该方案存在 TOCTOU 风险，可能在引用已删除权限时失败于更晚阶段或破坏明确错误和回滚边界，因此不采用。

### Decision: 用真实 PostgreSQL fixture 断言查询次数

permission PostgreSQL store 测试覆盖空、单个、多个、重复、数据库乱序返回和任一缺失。100/1000 规模测试通过真实 PostgreSQL fixture 创建测试权限，并在 fixture 完成后重置测试专用 SQL 查询计数器，只统计被测 `GetByPermissionIDs` 产生的 permission lookup SELECT，两个规模均断言为 1；测试辅助 instrumentation 只存在于测试代码，不增加生产接口或分支。

role command 测试使用更新后的 mock 明确期望 `GetByPermissionIDs` 恰好一次，并断言 lookup 失败时 `RolePermissionStore.Replace` 为零次、成功时 Replace 收到的顺序等于首次出现顺序。role PostgreSQL adapter 测试验证批量映射和 `ErrPermissionNotFound` 传播。生成 mocks 与 Fx 测试替身随端口签名同步。

仅通过返回集合长度无法证明 SQL 次数恒定，因此必须保留显式查询计数断言；仅使用 SQLite 也不能覆盖要求的 PostgreSQL `IN` 查询路径。

## Risks / Trade-offs

- [Risk] PostgreSQL `IN` 不保证返回顺序，直接映射会产生非确定结果 -> Mitigation：始终按去重输入 ID map 重排，并用反向数据库返回顺序测试覆盖。
- [Risk] 查询返回部分权限时错误地继续替换会删除合法旧绑定 -> Mitigation：重排期间发现任一缺失即返回 `ErrPermissionNotFound` 和 nil 结果，command 测试断言 Replace 与通知均未调用。
- [Risk] 为性能测试创建 1000 条 fixture 会放大测试耗时 -> Mitigation：在单个 PostgreSQL 容器和批量 fixture 准备范围内复用连接，查询计数只包围被测调用，不修改生产基线。
- [Risk] 端口扩展会使 mocks、Fx 测试替身或其他实现未同步而编译失败 -> Mitigation：通过 `go generate ./...` 更新生成 mock，并运行相关包测试及 `make verify`。
- [Trade-off] 完整路径仍执行两次批量权限查询 -> 这是 application 快速拒绝与事务内 TOCTOU 防护各自承担的职责；查询次数为常数，不能以减少一次查询为由删除事务内重校验。

## Migration Plan

1. 同一版本内先扩展 permission 与 role application 端口及其实现，再切换 `ReplaceRolePermissions` 调用并同步 mocks、测试替身和测试。
2. 运行相关 package 测试、架构 lint、OpenSpec validate，再暂存本次预期变更并执行 `make lint` 与 `make verify`。
3. 本变更不涉及数据迁移、配置、部署清单或公开 API，可随普通 user-service 版本滚动发布。
4. 如需回滚，整体回退本 change 的 Go 代码和规格即可；无需回滚数据库、OpenAPI 或部署资产。回滚会恢复 O(N) application 查询，因此仅用于代码级故障处置。

## Verification

- `go test ./user-service/internal/features/permission/infrastructure/postgres`
- `go test ./user-service/internal/features/role/application/command`
- `go test ./user-service/internal/features/role/infrastructure/postgres`
- `make user-service-architecture-lint`
- `openspec validate optimize-role-permission-bulk-lookup`
- 在暂存全部预期变更后运行 `make lint` 和 `make verify`
- 检查 git diff，确认没有 Ent、migration、OpenAPI、部署或无关文件变更。

## Open Questions

无。
