## Context

user-service 当前把权限同时建模为可由管理员维护的目录、带 `active/is_system` 状态的数据库实体，以及由 `rbacbaseline.DefaultPermissions()` 引导的系统数据。公开 permission command、route diff query、Casbin loader、role binding 校验、Ent schema、OpenAPI 和观测装配都依赖这套模型。多权威来源允许数据库内容偏离真实 Gin route graph，并使停用权限成为影响授权的额外运行时状态。

本次变更跨越 permission、role、`internal/shared/rbacbaseline`、router/providers、Ent/Atlas、OpenAPI、E2E 数据和文档规格，且会删除公开 API 和数据库字段。权限仍是 PostgreSQL 中可被 `role_permissions` 引用的实体，但其业务权威来源改为代码基线，数据库只保存 seed 后的查询和授权投影。角色及其 `Active`、`IsSystem` 语义、动态角色权限绑定、动态用户角色绑定、Casbin 的 HTTP method + route template 模型和现有多副本 policy sync 均继续保留。

代码归属保持现有边界：稳定权限定义留在 `user-service/internal/shared/rbacbaseline`，permission feature 拥有查询、授权和持久化 adapter，role application 只依赖 permission application 暴露的最小 lookup port。该业务语义不得下沉至 `common`，也不引入 `internal/integration`、部署运行时自动 migration 或新的外部依赖。

## Goals / Non-Goals

**Goals:**

- 让 `rbacbaseline.DefaultPermissions()` 成为权限定义的唯一业务权威来源，并以稳定 `permission_id` seed 数据库投影。
- 将权限公开 API 收敛为列表和用户有效权限查询，彻底移除权限写入、详情、启停和运行时 route diff API。
- 从 Permission domain、application、transport、Ent schema、SQL、OpenAPI 和测试数据中移除 `Active/IsSystem`。
- 保留角色动态管理和角色/用户绑定，并允许普通角色绑定任意存在的代码基线权限。
- 继续仅从启用角色、关系表和 permissions 投影构造 Casbin policy。
- 用构建真实 Gin route graph 的自动化测试替代公开 route diff，确保受保护路由与代码基线双向一致。
- 提供可审查、可排序部署且能安全清理 6 条废弃权限和绑定的 Atlas migration。

**Non-Goals:**

- 不改变 `Role.Active`、`Role.IsSystem`、系统角色保护、用户角色绑定或超级管理员 wildcard policy。
- 不改变 Casbin subject、object、action 格式，不引入 Casbin rule 表作为权威来源。
- 不让 seed 自动删除基线之外的权限，也不在 HTTP 服务启动时执行 schema 或数据 migration。
- 不提供新的权限管理 UI、运行时 route diff endpoint、自动权限发现或自动修复能力。
- 不改变认证公开接口、会话控制接口及其授权旁路语义。
- 不修改 `common`、Kubernetes/Helm/Compose 运行时资源或观测 dashboard，除非删除 route diff 指标后现有资产存在直接引用。

## Decisions

### 1. 代码基线是唯一权限定义，数据库是关系投影

`rbacbaseline.DefaultPermissions()` 返回完整权限集合，权限 ID、名称、描述、模块、HTTP method 和 route template 均由代码拥有。`SeedPermissionInput` 只携带这些字段；seed store 将 `UpsertSystemPermission` 重命名为 `UpsertPermission`，按 `permission_id` upsert 全部可变描述字段。method 或 path 调整必须沿用原 ID，使 `role_permissions` 外键关系保持稳定。

选择该方案是因为路由权限随代码发布，代码审查和测试比在线管理员写入更适合作为权威入口。备选方案是保留数据库权威并只关闭 HTTP 写接口，但离线修改、状态字段和基线漂移仍会形成多个事实来源，因此不采用。

权限删除不由 seed 执行。删除基线项时必须提交受控 migration，先按稳定 ID 删除 `role_permissions`，再删除 `permissions`。这避免 seed 在误配置或部分发布时破坏角色授权。备选的 seed 自动 reconcile-delete 风险过高，也不符合受控数据迁移边界。

### 2. Permission 移除生命周期和系统数据语义

删除 Permission 的 `Active`、`IsSystem` 以及数据库 `active`、`is_system` 列和对应索引。权限只存在或不存在，不再有启停状态，也不区分系统/非系统权限。列表参数、response/mapper、predicate、validator、错误和 mock 同步收缩；`Role.Active` 与 `Role.IsSystem` 不受影响。

角色绑定 lookup 由“存在且 active”改为“存在”，相关 `getLockedActivePermission...` 命名改为普通 permission lookup。Casbin loader 删除 `permission.ActiveEQ(true)`，但继续使用 `role.ActiveEQ(true)`。有效权限查询也仅以启用角色和存在的权限关系聚合。

备选方案是保留字段但固定为 true，然而这会留下误导性 API/schema、无效索引和未来再次出现双重生命周期的可能，因此直接移除。

### 3. 权限 HTTP 边界严格收敛为两个查询接口

`RegisterRoutes` 只注册：

```go
func RegisterRoutes(group *gin.RouterGroup, controller *PermissionController) {
	group.GET("", controller.ListPermissions)
	group.GET("/users/:user_id/effective", controller.ListUserEffectivePermissions)
}
```

删除 `POST /permissions`、`GET /permissions/route-diff`、`GET /permissions/:permission_id`、`PUT /permissions/:permission_id`、`POST /permissions/:permission_id/enable` 和 `POST /permissions/:permission_id/disable` 的 controller、request、response、validator 调用、OpenAPI 注解和路由注册。permission command service 的 `CreatePermission`、`UpdatePermission`、`EnablePermission`、`DisablePermission` 及其 store 方法一并删除；先使用 `rg` 确认引用，role 仍消费的查询端口不得误删。

备选方案是保留 endpoint 并返回 `405` 或 `410`，但项目尚未正式上线且没有兼容需求，保留空壳会继续扩张公开契约，因此直接删除。

### 4. 路由一致性改为真实 route graph 的测试门禁

测试使用与生产相同的 router registrar 构建真实 Gin route graph，扫描 `/api/v1` 下需要 RBAC 授权的 route template + HTTP method 集合，并与 `rbacbaseline.DefaultPermissions()` 双向比较。实际路由无基线项为 missing，基线项无实际路由为 stale，两者均使测试失败。扫描排除 `OPTIONS`、认证公开接口和会话控制接口，且比较稳定 ID 对应的 method/path，防止仅集合相等却错误复用 ID。

测试能力应尽量复用简单、无业务副作用的 route 归一化逻辑；如果现有 `RouteCatalogScanner` 只服务于 HTTP route diff，则在测试迁移完成后删除其 production provider、application query、controller 和 metrics。不得仅为测试保留冗余生产接口或把 Gin engine 引入 application 层。

备选方案是在服务启动时校验并拒绝启动，但这会把发布错误推迟到运行环境，也增加 composition 生命周期风险；CI 测试能更早反馈且不改变运行时可用性，因此优先采用。

### 5. migration 显式完成 schema 与废弃权限清理

Ent schema 先删除两个字段和索引，再通过 `make user-service-generate` 更新生成代码。Atlas migration 必须按顺序：删除 6 个废弃 HTTP 路由权限对应的 `role_permissions`；删除对应 `permissions`；删除 `permission_active_permission_id`、`permission_is_system_permission_id` 索引；删除 `active`、`is_system` 列。SQL 使用稳定 permission UUID 定位数据，外键清理在父记录删除之前完成，并保持对重复执行或目标数据不存在的安全语义。

迁移通过独立受控流程在新 HTTP 版本 rollout 前执行，随后执行 `rbac seed` 更新剩余投影，最后滚动重启 HTTP 副本以加载最新 policy；若有正式显式 reload 入口，也可在 seed/migration 后调用。E2E 初始化 SQL同步为新 schema 与新基线。

不在普通 user-service 镜像或启动路径执行 migration，也不由 seed 删除 stale 权限。部署清单本身无需改变，发布顺序和文档需要保持一致。

### 6. 现有 policy sync 只响应仍存在的在线 RBAC 写操作

权限在线写操作消失后，policy notifier 不再由 permission command 使用。角色状态、角色权限绑定和用户角色绑定成功后，现有本实例 reload/cache invalidation、Redis policy version、Pub/Sub 和定时补偿语义继续不变。离线 migration 与 seed 不宣称触发在线 refresh，必须由显式 reload 或滚动重启收敛。

这样可避免为代码定义权限新增特殊运行时同步协议。备选方案是让 seed 发布 Redis 通知，但 CLI 当前不等同在线服务实例，接入运行时 Redis sync 会扩大运维命令依赖和失败模式，因此不采用。

### 7. 规格、文档和生成物作为同一变更交付

更新 `openspec/specs/rbac-access-control/spec.md` 的权限目录、角色绑定、有效权限、policy 来源、seed、错误、观测和架构语义；更新 `docs/PRODUCT.md` 与 `docs/ARCHITECTURE.md`，删除管理员维护权限和公开 route diff 描述。OpenAPI 通过服务脚本重新生成，禁止手写生成物。`common` 无业务代码变更；`deployments` 仅在现有 route diff metric 被 dashboard/alert 引用时做最小删除同步。

验证包括 permission/role/Casbin 单元测试、router 注册测试、真实 route graph 一致性测试、E2E、Ent/Atlas/OpenAPI 生成门禁、架构 lint 和完整 `make verify`。

## Risks / Trade-offs

- [代码基线与已部署数据库版本不匹配会短暂造成授权缺失] → migration 后先运行匹配版本的 `rbac seed`，再滚动 HTTP 副本；发布工件和 seed CLI 使用同一版本代码。
- [错误删除权限 UUID 会级联或显式清理合法角色绑定] → migration 使用审核过的 6 个稳定 UUID，先查询/测试 fixture 验证映射，显式先删 join rows，再删权限记录，并由 migration validation 覆盖。
- [删除 `active/is_system` 时遗漏生成代码、DTO 或 predicate] → 使用 `rg` 全仓扫描 Permission 相关 `Active/IsSystem`、列名和索引名，并运行 Ent 生成、编译、架构 lint 和完整测试；单独确认 Role 字段仍存在。
- [真实路由扫描排除规则不准确导致 false positive/negative] → 从生产 route registrar 构图，以明确 method/template 白名单表示认证与会话旁路，并用 missing、stale、`OPTIONS` 和稳定 ID 场景测试排除逻辑。
- [移除 route diff metrics 后 dashboard 或 provider 留下悬空依赖] → 删除前 `rg` 检查 scanner、query、metrics 的所有生产引用，必要时同步最小观测资产并运行对应 dashboard 检查。
- [公开 API 删除影响未记录消费者] → 项目未正式上线且本变更明确为 breaking；OpenAPI 删除契约并通过 router/OpenAPI 测试保证不再暴露，不保留兼容路由。
- [数据库只读投影仍可能被人工 SQL 修改] → 应用不提供写入口，CI 保证代码路由一致性，受控 seed 可恢复定义字段；数据库权限治理属于部署运维边界，本次不新增数据库级 trigger。

## Migration Plan

1. 在同一版本中完成代码、测试、Ent schema、OpenAPI 和文档变更，生成并审查 Atlas migration。
2. 在 CI 中运行 route graph/基线一致性测试、相关包测试、`make user-service-generate`、`make user-service-migrate-validate`、`make user-service-openapi-generate`、`make user-service-architecture-lint` 和 `make verify`。
3. 发布时先备份/确认当前 6 个权限 UUID 及绑定数量，通过独立 migration job 执行 join row、权限记录、索引和列删除。
4. 使用与待发布 HTTP 服务相同版本执行 `aegiscore-user-service rbac seed`，按稳定 ID upsert 剩余权限及默认绑定。
5. 滚动部署或重启 HTTP 副本，使 Casbin policy 从新投影重新加载；检查 readiness/startup、授权 deny/error metrics 和关键角色访问。
6. rollback 代码前必须先应用配套逆向 migration，恢复两列、索引和旧权限记录/绑定；由于被删除绑定无法从 schema 自动恢复，回滚工件需使用部署前备份或预先记录的数据。若无法恢复绑定，则不得单独回滚到依赖旧权限 API 的版本。

## Open Questions

无。6 个待清理权限的稳定 UUID 在实现阶段从当前 `rbacbaseline.DefaultPermissions()` 与现有 migration/fixture 中核对后写入受控 migration，不改变本设计决策。
