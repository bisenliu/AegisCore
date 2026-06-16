## Context

AegisCore 用户服务已经有角色、权限、用户角色绑定和角色权限绑定的数据模型，并通过 role 与 permission feature 分层提供 HTTP 管理能力。当前权限 feature 已存在 `application/catalog`，但缺少稳定 `permission_id` 字段；role feature 尚未提供默认系统角色 catalog。部署侧需要在 schema migration 完成后、HTTP server 启动前显式写入默认 RBAC 数据，避免服务启动时产生静默数据库副作用。

本变更只为用户服务新增显式 CLI/Makefile seed 工作流。实现需要遵守现有 feature-first 分层：catalog 位于对应 feature 的 application 层，业务编排位于 application use case，Ent/PostgreSQL upsert 和绑定写入位于 infrastructure adapter，Cobra 命令只做配置、参数解析和调用编排。

## Goals / Non-Goals

**Goals:**

- 提供 `aegiscore-user-services rbac seed`，可重复执行并幂等写入系统角色、系统权限和系统角色权限绑定。
- 提供 `make seed-rbac`，作为本地和部署脚本使用的统一入口。
- 使用代码 catalog 管理默认系统角色、默认系统权限和默认系统角色权限绑定。
- 默认保留人工停用的系统角色或权限，即不覆盖 `active=false`。
- 支持 `--reactivate-system` 显式恢复系统角色或权限启用状态。
- 支持 `--sync-system-bindings` 按 catalog 精确同步系统角色权限绑定。
- 提供 `aegiscore-user-services rbac assign-super-admin --user-id <uuid>`，显式给指定用户绑定超级管理员角色。
- 明确新增系统权限和系统角色的运营更新流程。

**Non-Goals:**

- 不在 HTTP server 启动时自动 seed。
- 不通过 migration 维护全部权限清单。
- 不自动删除 catalog 中移除的角色或权限。
- 不自动给普通自定义角色授权。
- 不自动绑定首个超级管理员用户。
- 不新增 `casbin_rules` 表。
- 不实现完整审计日志、菜单权限、多租户或角色继承。

## Decisions

### Decision: RBAC seed 作为独立 CLI 命令执行

`rbac seed` 由 Cobra 命令显式触发，并复用用户服务配置加载和 Ent/PostgreSQL provider 所需配置连接数据库。HTTP `serve` 命令不调用 seed，也不通过 Fx HTTP server lifecycle 间接触发 seed。

Rationale: seed 是部署和运维动作，不是服务运行时副作用。显式 CLI 能在 migrate schema 与 start HTTP server 之间稳定插入，也便于 CI/CD 失败重试。

Alternatives considered: 在 HTTP 启动时自动 seed。该方案减少部署步骤，但会隐藏写库副作用，且多个实例并发启动时难以区分初始化失败和服务启动失败。

### Decision: Catalog 归属各 feature application 层

默认角色 catalog 放在 `user-service/internal/features/role/application/catalog/`，默认权限 catalog 扩展现有 `user-service/internal/features/permission/application/catalog/`，系统角色权限绑定 catalog 放在 role application catalog 中，因为绑定以角色默认授权为 owner。

Rationale: catalog 是业务规则输入，不属于 transport、Ent schema 或 Redis/PostgreSQL adapter；按 feature 拆分可以维持 role 与 permission 边界。

Alternatives considered: 建立横向 `internal/rbac/catalog`。该方案会把 role 与 permission 的业务身份集中到横向包，弱化现有 feature-first 组织。

### Decision: Upsert 写入由 infrastructure adapter 实现，seed 编排由 application use case 持有

role 和 permission application 层定义 seed 所需的最小 port，例如 upsert 系统角色、upsert 系统权限、补齐或同步系统角色权限绑定、绑定超级管理员用户。PostgreSQL adapter 使用 Ent 或 SQL builder 实现具备唯一约束语义的幂等写入。

Rationale: application 层拥有 seed 流程和默认不覆盖 `active=false` 的业务策略；infrastructure 层拥有数据库唯一约束、内部 ID 查询和事务边界。

Alternatives considered: 在 CLI 命令中直接使用 Ent。该方案实现更短，但会把业务编排和持久化细节放入 `cmd`，破坏分层并降低可测试性。

### Decision: 默认 upsert 不覆盖 active=false

系统角色和系统权限已存在时，seed 更新名称、描述、模块、方法、路径模板和 `is_system=true` 等 catalog 管理字段，但默认不把 `active=false` 改回 `true`。只有传入 `--reactivate-system` 时才恢复系统 catalog 条目的启用状态。

Rationale: `active=false` 是权限废弃和应急停用的重要运营控制，默认 seed 不应撤销人工停用。

Alternatives considered: 每次 seed 都把 catalog 条目强制置为 active。该方案保证 catalog 是强声明状态，但会覆盖运营停用，风险较高。

### Decision: 默认只补齐系统角色权限绑定，同步删除需显式开启

`rbac seed` 默认只补齐 catalog 声明但数据库缺失的系统角色权限绑定，不删除数据库中额外绑定。传入 `--sync-system-bindings` 时，seed 才按 catalog 精确同步系统角色权限绑定并移除多余的系统角色权限绑定。

Rationale: 默认补齐最符合安全渐进更新，避免因为 catalog 漏项导致系统角色突然丢权限；精确同步适合经过审核的发布流程。

Alternatives considered: 默认精确同步。该方案可减少权限漂移，但对 catalog 完整性要求更高，误删风险更大。

### Decision: 超级管理员用户绑定单独命令

`rbac assign-super-admin --user-id <uuid>` 使用已存在的 super admin role catalog ID，把指定用户绑定到超级管理员角色。该命令不由 seed 自动调用。

Rationale: 用户身份是环境数据，不属于系统 catalog；自动绑定会引入危险默认授权。

Alternatives considered: seed 时读取配置自动绑定首个管理员。该方案便捷，但容易在错误环境中产生高权限授权。

## Risks / Trade-offs

- [Risk] Catalog 与真实受保护路由不一致导致权限缺失或过期权限残留 -> Mitigation: seed 后运行现有 route-diff，检查 MissingInPermissions 和 StalePermissions。
- [Risk] 并发执行 seed 时出现唯一约束冲突 -> Mitigation: 依赖 `role_id`、`permission_id`、`http_method + path_template` 和绑定唯一约束实现数据库级幂等 upsert。
- [Risk] `--sync-system-bindings` 误删系统角色绑定 -> Mitigation: 默认关闭，仅在发布流程确认 catalog 完整时显式使用，并限制同步范围为系统角色 catalog 管理的绑定。
- [Risk] `--reactivate-system` 恢复了原本被人工停用的敏感权限 -> Mitigation: 默认关闭，并在命令输出中报告被恢复启用的条目数量。
- [Risk] CLI 直接复用完整 Fx app 会启动 HTTP server -> Mitigation: seed 命令应使用独立 seed runner/provider，只初始化配置、日志和数据库访问所需资源。

## Migration Plan

1. 添加或完善 role、permission 和 role-permission catalog，给所有系统权限写入稳定 `permission_id`。
2. 添加 seed application use case 和 PostgreSQL adapter 方法，覆盖幂等 upsert、默认保留 `active=false`、补齐绑定和同步绑定选项。
3. 添加 `rbac seed` 与 `rbac assign-super-admin` CLI 命令，并添加 `make seed-rbac` 包装。
4. 更新部署流程为 migrate schema -> seed RBAC data -> start HTTP server。
5. 新增系统权限时，更新 permission catalog，执行 `make seed-rbac`，运行 route-diff 验证 MissingInPermissions 和 StalePermissions，再触发 policy reload。
6. 新增系统角色时，更新 role catalog 和必要的 DefaultRolePermissions，执行 `make seed-rbac`，再触发 policy reload。

Rollback strategy: 回滚代码后不自动删除已 seed 的系统角色、权限或绑定。需要撤销权限时优先通过现有接口或 SQL 将权限 `active=false`，并触发 policy reload；稳定后再考虑人工物理删除。

## Open Questions

- policy reload 的具体触发机制当前不在本变更范围内；本变更只要求 seed 和运营流程在需要时执行或触发 reload。
