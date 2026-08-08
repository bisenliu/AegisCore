## Purpose

定义 user-service 的 RBAC 访问控制能力，覆盖权限目录、角色与绑定、Casbin 授权、策略同步、系统数据引导、统一错误、观测和资源生命周期。
## Requirements
### Requirement: 权限目录、路由门禁与默认基线

系统 MUST 将 `internal/shared/rbacbaseline.DefaultPermissions()` 作为权限定义的唯一业务权威来源，并将 permissions 数据库表作为供查询、角色绑定和授权加载使用的只读投影。权限 MUST 使用稳定 `permission_id` 描述可授权的 HTTP method、route template 和业务模块；运行时 MUST NOT 提供权限创建、详情、更新、启停或 route diff 公开接口。系统 MUST 在 `internal/shared/rbacbaseline` 集中维护默认系统角色及其默认权限绑定；`DefaultRoles()`、`DefaultPermissions()` 和 `DefaultRolePermissions()` 的公开函数签名 MUST 保持稳定，默认绑定 MUST 只引用代码基线中的稳定 ID。

#### Scenario: 权限查询契约

- **WHEN** user-service 注册权限 HTTP 路由
- **THEN** 系统 MUST 只注册 `GET /api/v1/permissions` 和 `GET /api/v1/permissions/users/:user_id/effective`
- **AND** 系统 MUST NOT 注册权限创建、详情、更新、启用、停用或 route diff HTTP 路由
- **WHEN** 授权调用方查询权限目录
- **THEN** 系统 MUST 按稳定权限 ID 排序返回完整匹配权限投影集合
- **AND** 请求 MUST 只支持 `module` 和 `http_method` 过滤参数，MUST NOT 接受或展示 `cursor` 或 `page_size`
- **AND** 成功响应 MUST 使用 `data.items` 包装集合，MUST NOT 包含 `data.pagination`
- **AND** 输入和响应 MUST NOT 包含 `active`、`is_system` 或 `system`
- **WHEN** `http_method` 非法
- **THEN** 系统 MUST 返回 `400 Bad Request`

#### Scenario: 权限投影维护与受控删除

- **WHEN** 运维执行 RBAC seed
- **THEN** 系统 MUST 按 `DefaultPermissions()` 中的稳定 `permission_id` 幂等 upsert 权限名称、描述、模块、HTTP method 和 route template
- **AND** method 或 route template 修改 MUST 沿用原 `permission_id`，使已有角色权限绑定保持不变
- **AND** 权限实体、seed 输入和数据库投影 MUST NOT 包含权限启停或系统权限标记
- **WHEN** 权限从 `DefaultPermissions()` 删除
- **THEN** 受控 migration MUST 先删除对应 `role_permissions` 再删除 `permissions` 记录
- **AND** seed 和 HTTP 运行时 MUST NOT 自动删除基线之外的权限或绑定
- **AND** 清理后系统 MUST 通过显式 policy reload 或滚动重启使 Casbin policy 收敛

#### Scenario: 路由与权限基线一致性门禁

- **WHEN** CI 或测试构建真实 Gin route graph 并扫描 `/api/v1` 下需要 RBAC 授权的路由
- **THEN** 系统 MUST 将 HTTP method 和 route template 与 `DefaultPermissions()` 双向比较
- **AND** 任一实际路由缺少基线权限或任一基线权限没有对应实际路由时测试 MUST 失败
- **AND** 扫描 MUST 排除 `OPTIONS`、认证公开接口和会话控制接口
- **AND** 校验 MUST NOT 创建或修改权限、角色绑定或运行时 policy

#### Scenario: 默认系统角色权限基线

- **WHEN** 代码调用 `DefaultRoles()` 和 `DefaultRolePermissions()`
- **THEN** `DefaultRoles()` MUST 仍只返回内置超级管理员角色，`DefaultRolePermissions()` MUST 返回该角色到全部默认权限的无重复绑定
- **AND** 每条绑定的 `RoleID` 和 `PermissionID` MUST 分别引用 `DefaultRoles()` 与 `DefaultPermissions()` 中的已知 ID
- **WHEN** 后续新增非超级管理员默认系统角色
- **THEN** 该角色的默认权限 MUST 在角色 catalog block 中显式列出 `PermissionID`
- **AND** 系统 MUST NOT 按 `Module`、model、read/write、路由前缀或其他粗粒度集合自动推导，也 MUST NOT 引入 `PermissionSet` 别名层
- **AND** 自动绑定全部 `DefaultPermissions()` 的内部 helper MUST 只用于超级管理员

### Requirement: 角色、权限与用户绑定

系统 MUST 提供角色创建、更新、启停、详情、列表和角色权限绑定，以及用户角色绑定的查询、添加、移除和完整替换能力。公开写接口 MUST NOT 允许创建或篡改系统角色；普通角色 metadata、状态和权限绑定写端口 MUST 在同一数据库事务内锁定目标角色并基于最新 `IsSystem` 拒绝系统角色。绑定 MUST 只引用存在的代码基线权限、未软删除用户和启用角色，完整替换 MUST 保持事务性。角色权限完整替换 MUST 在 application 层批量校验去重后的完整权限集合，且权限校验查询次数 MUST NOT 随权限 ID 数量增长。

#### Scenario: 角色目录写入和查询

- **WHEN** 授权调用方提交合法角色信息和存在的权限集合
- **THEN** 系统 MUST 创建非系统角色并写入角色权限绑定，成功响应 MUST 返回新建角色实体
- **WHEN** 授权调用方更新、启用或停用存在的普通角色
- **THEN** 系统 MUST 持久化变更，成功响应 MUST NOT 包含更新后的角色实体
- **WHEN** 授权调用方查询角色详情或分页查询角色
- **THEN** 系统 MUST 返回角色数据、权限摘要和共享 pagination 信息

#### Scenario: 系统角色 metadata 与状态保护

- **WHEN** 普通角色接口尝试修改系统角色的 `Name`、`Description` 或 `Active`，或提交与当前值相同的 metadata 或状态
- **THEN** 系统 MUST 返回 `roledomain.ErrSystemRoleProtected` 语义且 HTTP 接口 MUST 返回 `409 Conflict`
- **AND** 系统 MUST 保持该角色全部字段、角色权限绑定和系统基线语义不变
- **WHEN** 公开角色创建接口创建角色
- **THEN** 持久化记录的 `IsSystem` MUST 为 `false`，公开输入 MUST NOT 提供提升为系统角色的方式

#### Scenario: 系统角色权限绑定保护

- **WHEN** 角色权限 add、replace 或 remove 普通写请求以系统角色为目标并引用合法基线权限
- **THEN** 系统 MUST 返回 `roledomain.ErrSystemRoleProtected` 语义且 HTTP 接口 MUST 返回 `409 Conflict`
- **AND** 系统 MUST 保持该系统角色的已有权限绑定集合不变
- **WHEN** 角色权限写请求引用不存在或不属于当前代码基线投影的权限
- **THEN** 系统 MUST 拒绝写入并保持已有关系不变
- **AND** role application MUST 通过 permission application 拥有的最小查询端口校验权限，MUST NOT 导入 permission infrastructure
- **WHEN** 调用方把任意现存基线权限绑定给普通角色
- **THEN** 系统 MUST 允许绑定且 MUST NOT 检查权限 active 或 system 状态
- **AND** Permission 状态语义的移除 MUST NOT 删除或改变 `Role.Active` 与 `Role.IsSystem`

#### Scenario: 系统角色保护的事务原子性

- **WHEN** 普通 metadata、状态或角色权限 store 开始目标角色写事务
- **THEN** store MUST 在任何角色或绑定 mutation 前以 PostgreSQL `FOR UPDATE` 锁定目标角色，并以锁定后的最新 `IsSystem` 判定是否允许写入
- **AND** metadata UPDATE MUST 额外强制 `is_system=false`，application 层事务外读取 MUST NOT 作为系统角色保护的权威判断
- **WHEN** store 判定目标为系统角色并返回 `ErrSystemRoleProtected`
- **THEN** 本次事务 MUST NOT 改变角色、角色权限绑定、policy revision counter、policy revision 或 outbox event
- **AND** application MUST NOT 发送 policy change 通知或触发本实例 reload

#### Scenario: 系统角色保护与并发 seed

- **WHEN** RBAC seed 正在更新目标系统角色并持有该角色数据库行锁，普通 metadata、状态或角色权限写请求并发到达
- **THEN** 普通写请求 MUST 等待 seed transaction 结束并读取提交后的最新角色状态
- **AND** seed 提交 `IsSystem=true` 后普通写请求 MUST 返回 `ErrSystemRoleProtected`
- **AND** 普通写请求 MUST NOT 覆盖 seed metadata、改变系统角色绑定、推进 revision 或创建 outbox event
- **WHEN** 系统维护代码写入系统角色或其基线权限绑定
- **THEN** 系统 MUST 只使用 `SeedRoleStore` 或 `SeedRolePermissionStore` 受信端口，普通 HTTP use case MUST NOT 获得绕过参数、兼容开关或受信端口依赖

#### Scenario: 角色权限完整替换的 application 批量校验

- **WHEN** 调用方以一个或多个 permission ID 完整替换角色权限
- **THEN** role application MUST 先按首次出现顺序去重，并通过 permission application 查询端口一次性校验整个权限集合
- **AND** permission PostgreSQL store MUST 只执行一次 `WHERE permission_id IN (...)` 查询，100 个与 1000 个 permission ID 的 permission lookup SQL 查询次数 MUST 均为 1
- **AND** 系统 MUST NOT 依赖 PostgreSQL `IN` 查询的自然返回顺序，成功结果及传给绑定替换的权限顺序 MUST 与去重后的输入顺序一致
- **WHEN** 批量校验的任一 permission ID 不存在
- **THEN** 系统 MUST 返回 `permissiondomain.ErrPermissionNotFound` 语义且 MUST NOT 返回部分成功结果
- **AND** role application MUST NOT 调用角色权限绑定替换或发送 policy change 通知

#### Scenario: 空角色权限集合的批量校验

- **WHEN** 批量权限查询收到空 permission ID 集合
- **THEN** 系统 MUST 返回非 nil 空权限集合且 MUST NOT 访问数据库
- **AND** role application MUST 继续以空集合执行合法的角色权限完整替换

#### Scenario: 角色权限替换、停用与 seed

- **WHEN** 调用方以合法权限集合完整替换角色权限
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定，任一非幂等错误 MUST 回滚全部变更
- **AND** application 批量校验与事务写入之间任一权限变为不存在时，事务内重校验 MUST 拒绝替换并保持已有关系不变
- **WHEN** 普通角色被停用
- **THEN** 该角色 MUST NOT 出现在用户有效角色、有效权限或 Casbin policy 中
- **WHEN** seed 补齐或同步系统角色权限绑定
- **THEN** 系统 MUST 批量维护绑定并将已有绑定视为幂等成功，非唯一冲突错误 MUST 使本次事务失败

#### Scenario: 用户角色绑定与完整替换

- **WHEN** 调用方将存在且启用的角色绑定给存在且未软删除的用户
- **THEN** 系统 MUST 写入用户角色关系并使后续授权能够使用该角色
- **WHEN** 写请求引用不存在或已软删除的用户、不存在的角色或已停用角色
- **THEN** 系统 MUST 拒绝写入并返回明确错误
- **AND** 系统 MUST NOT 改变已有关系、失效缓存或发送 policy change 通知
- **WHEN** 调用方以全部合法且启用的角色集合完整替换用户角色
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一角色不可用或任一写入失败时系统 MUST 回滚全部变更

#### Scenario: 有效权限聚合

- **WHEN** 系统或授权调用方查询用户有效权限
- **THEN** 系统 MUST 聚合该用户当前启用角色及其绑定的存在权限并返回去重集合
- **AND** 响应 MUST NOT 包含权限 `active`、`is_system` 或 `system`
- **AND** 角色、权限、用户角色和角色权限 MUST 使用外部 UUID 作为稳定业务标识，join 表内部标识 MUST NOT 暴露给 application 或 transport
- **WHEN** 已认证用户没有有效角色绑定并访问受保护路由
- **THEN** 系统 MUST 拒绝访问

### Requirement: Casbin 授权、策略缓存与多副本同步

系统 MUST 在认证后使用 RBAC 中间件保护权限、角色和用户业务接口，并以 PostgreSQL 关系数据作为业务权威来源。系统 MUST 将会改变 Casbin 静态授权规则集合的事实记录为单调 policy revision，并将用户角色绑定变更记录为独立 user-role revision 或等价提交水位；两类 revision MUST NOT 混用。每个本地 Casbin enforcer MUST 与其实际加载的数据库 policy revision 绑定，applied revision MUST 表示该 engine 当前成功应用的授权投影，MUST NOT 表示 Redis 通知序号、用户角色绑定提交水位、消息接收进度或 reload attempt。授权 MUST 使用稳定 subject、Gin route template 和 HTTP method，并在任何身份、策略、revision 或执行异常下 fail-closed。Redis 与 Pub/Sub MUST 只传播数据库提交事实并加速副本收敛；授权热路径 MUST 使用本地投影，MUST NOT 每请求读取 Redis 或 PostgreSQL revision。

#### Scenario: 授权请求与超级管理员

- **WHEN** 已认证请求进入受保护的 `/api/v1` 路由
- **THEN** 中间件 MUST 使用请求上下文中的用户 ID、Gin `FullPath()` 和 HTTP method 构造授权请求
- **AND** 用户与角色 subject MUST 分别使用 `user:<user_uuid>` 和 `role:<role_uuid>`
- **WHEN** 用户当前启用角色拥有匹配 method 和 route template 的权限
- **THEN** 系统 MUST 允许请求进入 controller，否则 MUST 返回禁止访问错误
- **WHEN** 用户拥有 `rbacbaseline` 定义的内置超级管理员角色
- **THEN** policy loader MUST 为该角色提供受保护业务接口的 wildcard policy，超级管理员角色常量 MUST 只由 `rbacbaseline` 提供

#### Scenario: fail-closed 与路由注册

- **WHEN** 请求缺少用户 ID、用户 ID 类型非法或 subject 不能解析为用户 UUID
- **THEN** 系统 MUST 返回未认证错误并拒绝请求，且 MUST NOT 调用 Casbin engine
- **WHEN** 用户角色回源失败、context 取消、连续失效、Casbin 执行错误、policy 未加载、目标 policy revision 未追平或最近一次 policy reload 失败
- **THEN** 系统 MUST 拒绝请求并暴露 policy 不可用 readiness/startup 状态，MUST NOT 使用旧角色集合或保留的旧 enforcer 继续允许请求
- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS`
- **THEN** 中间件 MUST 允许请求并 MUST NOT 调用授权服务
- **WHEN** 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 中间件链；token version validator、RBAC authorizer 或必需 route registrar 缺失时系统 MUST 拒绝注册部分路由

#### Scenario: revision-aware policy 快照加载

- **WHEN** policy loader 面向目标数据库 policy revision 构造授权策略
- **THEN** loader MUST 在同一 PostgreSQL 一致性快照中读取可见 latest policy revision 与启用角色、角色权限绑定和 permissions 投影，并返回 `PolicySet{Revision, PermissionRules}` 或等价结构
- **AND** 返回的 revision MUST 大于或等于目标 policy revision，规则 MUST 与该 revision 所属数据库快照绑定；loader MUST NOT 为旧规则附加较新的 revision
- **AND** 用户身份解析 MUST 排除已软删除用户，loader MUST NOT 使用权限 active predicate，独立 `casbin_rules` 表 MUST NOT 成为业务权威来源
- **WHEN** 当前快照可见 policy revision 低于目标 policy revision
- **THEN** loader MUST 在 context 期限内结束旧快照并使用新快照重试，MUST NOT 返回低于目标的 policy、在旧快照内无限等待或将通知 revision 直接作为快照 revision
- **WHEN** target policy revision 为 0 且数据库尚无 policy revision 记录
- **THEN** loader MUST 以 revision 0 加载当前基线投影，并保持超级管理员 wildcard policy 语义
- **WHEN** 只有用户角色绑定发生变化且未改变角色状态、角色权限绑定或 permissions 投影
- **THEN** 系统 MUST NOT 调用 policy loader，MUST NOT 查询 `role_permissions` 全集，MUST NOT 构造新的 Casbin enforcer

#### Scenario: revision-aware engine 交换与防倒退

- **WHEN** engine 收到目标 policy revision 并完成候选 `PolicySet` 与 enforcer 构造
- **THEN** engine MUST 在同一锁定临界区比较候选 revision 与当前 applied revision，并原子交换 enforcer、applied revision 与成功状态
- **AND** 只有更高候选 revision 可以替换当前 enforcer，相等候选 MUST 幂等成功，较低候选 MUST 被丢弃且不得覆盖或降低当前投影
- **WHEN** policy revision 1 的 reload 在 policy revision 2 成功应用后才完成
- **THEN** 最终 enforcer 和 applied revision MUST 仍对应 policy revision 2 或更高的数据库快照
- **AND** engine、tracker/status、metrics 与 health 暴露的 applied revision MUST 来自同一实际投影状态，MUST NOT 由 watcher 独立推进
- **WHEN** user-role revision 高于当前 Casbin applied policy revision
- **THEN** engine applied revision MUST 保持不变，MUST NOT 把 user-role revision 当成 policy target 或 applied revision

#### Scenario: 同实例并发 reload 收敛

- **WHEN** 同一实例并发收到多个数据库 policy target revision
- **THEN** engine MUST 串行化或 coalesce 实际 reload 工作，将 pending target 单调提升到已观察到的最大 policy revision，并防止并发构造导致投影倒退
- **AND** 等待方只有在实际 applied revision 不低于其 target policy revision 时才能观察到成功；单个等待方 context 取消 MUST NOT 取消其他调用仍需要的共享 reload
- **WHEN** 100 个并发 policy 写入触发 reload 且数据库 latest policy revision 可见
- **THEN** reload 稳定后 engine applied revision MUST 等于加载时数据库 latest policy revision 且不低于全部 policy target 中的最大值
- **AND** 系统 MUST NOT 要求 policy revision 连续或为每个中间 policy revision 分别构造 enforcer

#### Scenario: 初始加载、reload 失败与恢复

- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 使用可取消或带超时的启动 context 显式加载当前数据库 latest policy revision
- **WHEN** 初始加载失败、被取消或不能达到目标 policy revision
- **THEN** engine MUST 保留实际 applied revision、记录最近错误和 reload 失败指标，后续授权 MUST fail-closed，`app.Start` MUST 保持成功
- **AND** reload 状态和 readiness/startup MUST 保留失败信息并拒绝接入业务流量
- **WHEN** 已存在成功投影后的 reload 加载、构造或交换失败
- **THEN** engine MUST 保留上一成功 enforcer 及其 applied revision，MUST NOT 提升 revision、清除失败或使用旧投影放行请求
- **WHEN** 后续显式 reload、Pub/Sub 或周期补偿成功应用不低于目标的数据库 policy 快照
- **THEN** engine MUST 原子替换或确认当前投影、清除最近 reload 错误并恢复 readiness/startup

#### Scenario: 用户角色缓存键、容量与值隔离

- **WHEN** user-role cache 启用
- **THEN** permission feature MUST 在 localcache 边界使用 `userID.String()` 作为规范 string key，并将配置的正数 size 映射为最大 item 数
- **AND** common MUST NOT 接收 key encoder、解析业务 UUID 或暴露底层 cache option
- **WHEN** 缓存命中
- **THEN** loader 写入缓存前和 `RolesForUser` 返回前 MUST 复制 `[]uuid.UUID`，调用方修改返回 slice MUST NOT 污染缓存或后续读取
- **AND** `common/runtime/localcache` MUST NOT 承担角色 ID clone 语义

#### Scenario: 用户角色回源与缓存失效

- **WHEN** 缓存未命中
- **THEN** 系统 MUST 合并同一用户的并发回源并查询 PostgreSQL 中的当前启用角色，loader 错误 MUST NOT 写入缓存
- **WHEN** 回源失败或结果连续两次与失效并发
- **THEN** 授权 MUST fail-closed，MUST NOT 因 cache 不可用产生允许结果
- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice 并保持 fail-closed；direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次结果

#### Scenario: 在线写后同步与数据库 revision 目标

- **WHEN** 角色状态、角色权限或权限投影通过在线 API 或受控维护流程与 policy revision 原子提交成功
- **THEN** 本实例 coordinator MUST 使用该数据库 policy revision 作为 reload 目标，outbox dispatcher MUST 传播同一数据库 policy revision
- **AND** reload 或通知失败 MUST 保持可诊断和 fail-closed 语义，cache invalidation MUST 保持同步幂等，MUST NOT 把通知接收、Redis publish 成功或 user-role revision 标记为 engine 已应用
- **AND** `PolicyChangeNotifier` 或等价 policy reload port MUST 是正式 command service 的必需依赖并接收数据库 policy revision
- **WHEN** 用户角色绑定通过在线 API 与 user-role revision 原子提交成功
- **THEN** 本实例 coordinator MUST 只执行指定用户角色缓存失效，outbox dispatcher MUST 传播同一 user-role revision 和 user ID，MUST NOT 调用 Casbin policy reload 或 policy loader
- **AND** user-role cache invalidation 失败 MUST 保持可诊断和 fail-closed 语义，MUST NOT 推进 Casbin target revision 或 applied revision
- **WHEN** 权限投影由离线 migration、seed 或 bootstrap 改变
- **THEN** 离线命令 MUST NOT 宣称已完成在线 policy refresh，运维 MUST 显式创建/传播对应 policy revision、执行 revision-aware reload 或滚动重启副本

#### Scenario: watcher、重复通知与副本收敛

- **WHEN** watcher 通过 Pub/Sub 或周期性检查发现数据库 policy revision 高于 engine applied revision
- **THEN** watcher MUST 以该 policy revision 调用 revision-aware application port，只有 engine 成功应用不低于 target 的投影后才能将该 policy revision 视为 applied
- **AND** Pub/Sub 丢失时周期性 policy revision 补偿 MUST 使副本最终收敛
- **WHEN** watcher 收到重复、相等或乱序 policy 通知
- **THEN** policy reload MUST 保持幂等且不得倒退 enforcer；已应用且 projection ready 的 policy revision MUST 跳过全量 reload
- **AND** watcher MUST 合并当前待处理通知，对同批 policy 通知只针对最高未应用 policy revision 构造一次 Casbin enforcer
- **WHEN** watcher 收到 user-role 通知
- **THEN** watcher MUST 执行消息要求的用户角色 cache invalidation 副作用，MUST NOT 独立推进 Casbin engine applied revision 或伪造 policy reload 完成
- **AND** 定向 user-role invalidation 通知携带可用 user ID 时 MUST 失效该用户缓存；存在 revision gap、无法证明精确用户集合完整或缺少可用 user ID 时 MUST 失效全部用户角色缓存
- **WHEN** watcher 处理 100 条重复或连续通知且其中没有新的未应用 policy revision
- **THEN** policy loader 调用次数 MUST 有常数上界；纯 user-role 通知 MUST 产生 0 次 policy loader 调用

### Requirement: RBAC watcher 自恢复生命周期与权威校准状态

RBAC watcher MUST 在单一显式生命周期内持续监督 Redis policy refresh 订阅与 PostgreSQL policy revision 权威校准。订阅故障 MUST NOT 终止数据库补偿；瞬时错误恢复后 MUST 更新当前状态且不得因历史错误保持永久失败。watcher MUST 只通过 permission application 拥有的结构化只读 status port 暴露状态，MUST NOT 保留 `Running()`/`LastError()` 旧接口、旧状态 adapter 或兼容分支。watcher 启动 MUST 接收显式 lifecycle context，并以该 context 派生后台运行 context；Start 路径 MUST NOT 使用 `context.Background()` 作为运行根上下文，也 MUST NOT 保留无参 `Start()` 兼容接口或 adapter。

#### Scenario: Watcher 生命周期与状态恢复

- **WHEN** permission runtime 启动、停止或启动回滚
- **THEN** lifecycle MUST 在 `OnStart(ctx)` 中显式调用 `Watcher.Start(ctx)`，或幂等调用 `Stop(ctx)` 停止 watcher，constructor MUST NOT 提前启动 goroutine
- **AND** watcher `Start(ctx)` MUST 使用传入 ctx 派生后台运行 context，并保存 cancel 供 `Stop(ctx)` 触发退出
- **AND** watcher MUST 在 stop context 内等待 in-flight 消息处理或 revision check 退出，`Stop(ctx)` 的 ctx MUST 只控制等待退出的期限，MUST NOT 替代 Start 建立的运行根 context
- **AND** watcher MUST NOT 关闭共享 Redis client
- **WHEN** 同一 watcher 已经运行且调用方再次调用 `Start(ctx)`
- **THEN** watcher MUST 保持单一后台 loop 或 ticker，并 MUST NOT 覆盖正在运行实例的 cancel 或启动第二个 worker
- **WHEN** Redis subscription 断开、重连或关闭消息 channel
- **THEN** watcher 根生命周期 MUST 保持 running，MUST NOT 要求人工操作、进程重启或新的 RBAC mutation 才能恢复

#### Scenario: Watcher 运行上下文观测

- **WHEN** watcher 启动、处理 Pub/Sub payload、执行周期 revision check、查询 latest revision、执行 projection reload 或记录同步结果
- **THEN** 后台消息处理、周期校准、结构化日志和 watcher metrics MUST 使用由 `Start(ctx)` 传入 lifecycle context 派生的运行 context 或其 logger-aware 派生 context
- **AND** watcher Start 路径 MUST NOT 调用 `context.WithCancel(context.Background())` 建立运行根 context

### Requirement: RBAC 系统数据与运维 CLI

系统 MUST 通过 `aegiscore-user-service` 根命令提供带服务上下文的 `rbac seed` 和一次性 `rbac bootstrap-super-admin`，用于维护系统基线和全新数据库的首次超级管理员引导。系统角色、系统权限、默认绑定和 bootstrap 用户 ID MUST 由 `internal/shared/rbacbaseline` 以手写保留 UUID 常量定义；普通运行时业务实体 MUST 继续使用 `common/runtime/id.NewUUID()` 生成 UUID v7。旧 `aegiscore-user-services` 根命令 MUST NOT 作为兼容入口保留。

#### Scenario: 初始化系统基线

- **WHEN** 运维执行 `aegiscore-user-service rbac seed`
- **THEN** 系统 MUST 幂等创建或更新基线角色、权限投影和绑定并输出变更统计
- **AND** 系统角色 MUST 保持系统标记，Permission MUST NOT 包含 `Active` 或 `IsSystem`
- **AND** seed MUST NOT 创建业务用户、自动分配超级管理员或自动删除基线之外的权限记录
- **AND** 非 seed 公开 HTTP 路径 MUST NOT 创建、修改或启停权限
- **AND** seed MUST 引用固化常量，MUST NOT 调用 `id.NewUUID()`、UUIDv5 或其他动态系统 ID 生成逻辑

#### Scenario: 系统 ID 编码、追加与稳定性

- **WHEN** 代码定义 `SuperAdminRoleID`、`BootstrapSuperAdminUserID` 或 baseline permission ID
- **THEN** 常量 MUST 位于 `user-service/internal/shared/rbacbaseline/ids.go`，并为手写固化 UUID 字符串，MUST NOT 由 UUIDv5、`go:generate` 或运行时代码生成
- **AND** 常量 MUST 匹配 `00000000-0000-0000-0000-TTMMSSSSSSSS`：`TT` 中 `01` 为系统用户、`02` 为系统角色、`03` 为系统权限、`09` 为测试、fixture 或诊断预留
- **AND** 系统用户和角色的 `MM` MUST 为 `00`；系统权限的正式模块编号 MUST 为 `01=user`、`02=permission`、`03=role`、`04=user-role`、`05=role-permission`
- **AND** `SSSSSSSS` MUST 在同一 `TT+MM` 下从 `00000001` 递增，MUST NOT 使用 `00000000`
- **AND** `ids.go` MUST 集中记录编码规则，每个常量注释 MUST 记录稳定 semantic
- **WHEN** 新权限模块首次进入 `DefaultPermissions()`
- **THEN** 系统 MUST 按首次进入顺序分配下一个正式 `MM`，已发布编号 MUST NOT 修改
- **AND** 系统 MUST NOT 提前为未来模块分配编号，`90` 至 `99` MUST 只用于测试、fixture 或诊断，MUST NOT 用于生产 baseline
- **WHEN** 系统 ID 已发布或对应 baseline 项被删除
- **THEN** 后续变更 MUST NOT 修改其值或将其复用于其他系统实体

#### Scenario: 系统 ID 引用与一致性门禁

- **WHEN** `DefaultPermissions()`、`DefaultRoles()` 或 `DefaultRolePermissions()` 返回基线
- **THEN** 每个 ID MUST 引用 `rbacbaseline` 常量，`DefaultPermissions()` MUST NOT 内联 UUID 字符串
- **AND** 测试 MUST 校验默认权限和绑定引用存在且不重复，并校验所有系统 ID 可解析、匹配 `^00000000-0000-0000-0000-[0-9]{12}$`、类型与模块登记正确、sequence 非零且全局唯一
- **AND** 全部 baseline permission ID MUST 登记在 `registeredPermissionIDs()` 并被默认权限和绑定校验覆盖
- **WHEN** 系统创建普通用户、普通角色或其他运行时数据
- **THEN** 创建路径 MUST 使用当前运行时 ID 生成策略，MUST NOT 复用系统 ID 或使用系统保留格式
- **WHEN** 系统创建或更新系统角色、bootstrap 用户、baseline permission 或默认绑定
- **THEN** 系统 MUST 使用固化常量，MUST NOT 使用 `id.NewUUID()`、UUIDv5、随机 UUID 或其他动态生成逻辑替代

#### Scenario: 超级管理员引导输入与固定标识

- **WHEN** 运维执行 `aegiscore-user-service rbac bootstrap-super-admin --username <name> --nickname <nickname> --password-env <env>`
- **THEN** `--username` MUST 必填且无默认值，并在 trim 后转小写；`--nickname` MUST 可选且 trim 为空时使用归一化 username
- **AND** `--password-env` 默认 MUST 为 `ADMIN_BOOTSTRAP_PASSWORD`，密码 MUST 只从该环境变量读取
- **AND** 密码 MUST NOT trim，长度 MUST 为 12 至 72 字节，首尾空格 MUST 作为密码内容参与校验和哈希
- **AND** CLI MUST NOT 提供直接密码、user ID、reset、force、reuse 或 reactivate 参数
- **WHEN** 系统执行首次引导
- **THEN** 系统 MUST 使用 `rbacbaseline.BootstrapSuperAdminUserID` 和 `rbacbaseline.SuperAdminRoleID`
- **AND** 固定用户 ID MUST NOT 被 CLI、环境变量或配置覆盖，bootstrap application MUST NOT 私有定义该 ID

#### Scenario: 事务性首次引导与重复拒绝

- **WHEN** bootstrap store 执行首次引导
- **THEN** 系统 MUST 在同一 PostgreSQL 事务中获取固定 advisory lock、查询超级管理员角色、检查固定用户 ID 与 username、创建固定 ID 用户并绑定角色
- **AND** 超级管理员角色 MUST 存在、`is_system=true` 且 `active=true`
- **AND** 固定用户 ID 查询 MUST 包含软删除用户，MUST NOT 添加 `deleted_at IS NULL`；username 占用检查 MUST 覆盖正常用户和软删除用户
- **AND** bootstrap 用户状态 MUST 为 `identity.UserStatusMustChangePassword`，密码 MUST 使用应用层传入的 bcrypt hash
- **AND** 任一步失败 MUST 回滚整个事务；唯一约束冲突 MUST 映射为稳定应用错误，MUST NOT 暴露 Ent 或 PostgreSQL 错误
- **WHEN** 固定用户 ID 已存在
- **THEN** 命令 MUST 以非零退出码拒绝，并返回 `super admin bootstrap has already been completed`
- **AND** 无论用户状态、软删除、角色或 username 是否变化，系统都 MUST 视为已完成，MUST NOT 修复、复用、重置或重新绑定

#### Scenario: 后续授权、旧入口与灾难恢复边界

- **WHEN** 首次 bootstrap 后需要授权其他超级管理员
- **THEN** 系统 MUST 使用在线用户角色绑定 API，由其完成校验、policy version 发布和缓存收敛
- **AND** 系统 MUST NOT 再次运行 bootstrap、提供离线密码重置、离线恢复或 `recover-super-admin`；全部超级管理员不可用时 MUST 只允许 DBA 人工介入或重新初始化数据库
- **WHEN** 调用 `rbac create-super-admin`、`rbac assign-super-admin`、`--reset-password` 或 `ADMIN_RESET_PASSWORD`
- **THEN** 系统 MUST 拒绝或忽略旧入口，MUST NOT 保留别名、双版本 CLI 或旧数据自动恢复行为

### Requirement: RBAC 错误契约与可观测性

permission、role 和 binding domain MUST 返回携带稳定 HTTP status、共享业务 code、公开 message 和 `Reason` 的应用错误，并保留 `errors.Is` 语义。HTTP transport MUST 通过共享 `response.Fail` 渲染业务错误，MUST NOT 维护 feature 专用 mapper。系统 MUST 提供低基数授权 metrics 和显式注入 logger；观测失败 MUST NOT 改变授权或同步结果。policy sync Redis prefix、Pub/Sub channel 和 metrics `service` label MUST 使用 `aegiscore-user-service`，MUST NOT 兼容旧 `aegiscore-user-services` prefix。

#### Scenario: 稳定错误映射与统一出口

- **WHEN** permission 查询返回权限不存在或输入无效
- **THEN** 系统 MUST 分别使用 `404`/`permission_not_found` 和 `400`/`permission_invalid`，MUST NOT 暴露权限已存在或系统权限保护写错误
- **WHEN** role 返回已存在、不存在、输入无效、系统角色保护或角色停用错误
- **THEN** 系统 MUST 分别使用 `409`、`404`、`400`、`409`、`409` 及 `role_already_exists`、`role_not_found`、`role_invalid`、`system_role_protected`、`role_inactive`
- **WHEN** 增量绑定已存在或不存在
- **THEN** 系统 MUST 分别返回 `409 Conflict` 或 `404 Not Found` 及对应稳定 `Reason`
- **WHEN** role 收到 `identity.ErrUserNotFound`、permission 错误，或 controller 收到业务错误
- **THEN** transport MUST 通过 `response.Fail(c, err)` 保留错误自身 status、code 和 message
- **AND** transport MUST NOT 复制跨 feature 映射或保留 `toPermissionHTTPError`、`toRoleHTTPError` 等 mapper

#### Scenario: 授权指标和日志

- **WHEN** authorization service 完成 Enforce
- **THEN** counter MUST 记录 `result="allow"`、`result="deny"` 或 `result="error"`，histogram MUST 记录耗时
- **AND** 标签 MUST 只使用 result、HTTP method 和 route template，默认 `service` MUST 为 `aegiscore-user-service`
- **AND** 指标 MUST NOT 包含用户、角色、权限、token、trace、IP、账号、Redis key、SQL、原始错误或 raw path
- **WHEN** RBAC 组件记录日志
- **THEN** logger MUST 由 constructor 显式注入或由调用方 context 提供
- **AND** 日志 MUST 使用稳定低基数字段，MUST NOT 记录 token、SQL、Redis key 或原始 policy 数据

#### Scenario: route diff 与同步命名空间

- **WHEN** user-service 组装生产 permission 模块
- **THEN** 系统 MUST NOT 注册 route diff handler 或装配专用 query、scanner、metrics
- **AND** 路由一致性 MUST 由 CI/测试失败表达，MUST NOT 自动修改权限投影
- **WHEN** Redis adapter 生成 policy version key 或 refresh channel
- **THEN** prefix MUST 来自当前 `app.name` 并归一化为 `aegiscore-user-service`
- **AND** adapter MUST NOT 读取、发布、订阅或迁移旧 prefix 下的 key 或 channel
- **AND** 副本收敛 MUST 依赖新 prefix 下的 version、Pub/Sub 和周期性补偿

### Requirement: RBAC 架构装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或 route diff 生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应边界。RBAC watcher、Casbin Engine policy reload flight 和 policy 投影主动资源 MUST 显式启动、停止和回滚；无后台执行的 user-role localcache MUST NOT 拥有启停或关闭生命周期。permission composition MUST 以单一 runtime 聚合对象表达稳定组件集合。

#### Scenario: 分层、bootstrap 与最小依赖

- **WHEN** role 或 permission application service 被构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明 DI tag
- **AND** 消费侧 application MUST 定义最小 port，feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport，role 仍使用的 permission lookup MUST 保留
- **WHEN** 实现超级管理员 bootstrap
- **THEN** application service MUST 位于 `user-service/internal/features/role/application/bootstrap/`，通过最小 `BootstrapStore` 调用 role infrastructure 中的 PostgreSQL adapter
- **AND** application MUST 负责输入归一化、密码策略、哈希及固定用户/角色 ID，MUST NOT 导入 Ent predicate、HTTP、Gin、Fx、SQL、Redis 或 datastore concrete implementation

#### Scenario: adapter、composition 与运行时聚合

- **WHEN** 构造 RBAC store、loader、engine、watcher、cache 或 adapter
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options，MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、named/group result
- **AND** named `primary_db`、`cache_redis`、optional、group 和 lifecycle 选择 MUST 留在 feature composition
- **AND** public provider MUST 只暴露 controller、authorizer、health/status、runtime 聚合对象和 application port，父 module MUST NOT 消费 infrastructure concrete implementation
- **WHEN** composition 提供 RBAC runtime 组件
- **THEN** composition MUST 通过单一 permission runtime 聚合对象表达已经构造的稳定接口或私有 lifecycle contract
- **AND** 聚合对象 MUST NOT 重建 engine、store、watcher、version tracker、cache、resolver、Redis client 或 Ent client，application/domain MUST NOT 依赖该对象

#### Scenario: 有状态资源单实例与必需依赖

- **WHEN** composition 暴露同一有状态组件的多个接口视图
- **THEN** 系统 MUST 只构造一个实例并以普通 Go 赋值暴露，MUST NOT 重复构造 engine、store、version tracker、watcher 或 cache
- **AND** watcher 的状态和运行器视图 MUST 指向同一实例
- **WHEN** 角色、角色权限或用户角色写侧服务装配
- **THEN** 服务 MUST 具备可用 notifier；缺少 notifier 或安全 collaborator 时 constructor MUST 返回明确 error 并拒绝装配，MUST NOT panic
- **AND** 系统 MUST NOT 用 no-op、nil fallback 或兼容 wrapper 跳过 reload、Redis version 或 watcher 同步

#### Scenario: watcher、Casbin Engine 与 cache lifecycle

- **WHEN** user-service 启停 permission/RBAC runtime
- **THEN** `NewWatcher` MUST 只构造对象，MUST NOT 启动 goroutine、订阅 Redis 或执行补偿循环
- **AND** hook MUST 先启动 Casbin Engine lifecycle root，再使用启动 context 执行 fail-closed 初始 policy 加载，然后启动 watcher；`Start()` 和 `Stop(ctx)` MUST 幂等，`Stop(ctx)` MUST 取消内部 context 并在调用方期限内等待 watcher 退出
- **AND** Stop 超时 MUST 返回 context 错误并保持重复停止安全，启动失败或停止时已启动 watcher MUST 被停止
- **WHEN** user-role localcache 被构造或应用停止
- **THEN** cache MUST 作为无后台 goroutine 的普通对象使用，Fx result 与 hook MUST NOT 为其提供或调用 `Start(context.Context) error`、`Close() error`、closed state 或 lifecycle rollback

#### Scenario: Casbin Engine shared reload flight root

- **WHEN** Casbin Engine 启动后创建 shared reload flight
- **THEN** `startFlightLocked` MUST 从 engine lifecycle root 派生 shared flight context，MUST NOT 从 `context.Background()` 或任一 waiter context 派生 shared flight context
- **AND** 任一单个 `ReloadToRevision(ctx)` 或 `RefreshToRevision(ctx)` waiter context 取消 MUST 只取消该调用等待，MUST NOT 取消其他 waiter 仍需要的 shared reload flight
- **AND** 全部 waiter 取消后 engine MAY 取消当前 shared reload flight 并保持 fail-closed，后续新 waiter MUST 能启动 fresh flight
- **WHEN** engine lifecycle root 被 RBAC lifecycle Stop、启动回滚或服务 shutdown 取消
- **THEN** 正在阻塞的 shared reload loader MUST 收到取消信号，engine MUST 记录 reload 失败并保持 fail-closed，MUST NOT 使用旧 `context.Background()` 兼容路径继续执行该 flight
- **WHEN** `InitializeFailClosed(ctx)` 执行启动期初始加载
- **THEN** engine MUST 使用 target revision 0 执行 reload；若启动 context 取消、loader 失败或目标 revision 未达到，服务启动 MUST 保持成功且后续授权 MUST fail-closed

#### Scenario: 共享资源所有权与关闭安全

- **WHEN** RBAC 关闭 watcher、store、Casbin Engine 或其他主动资源
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源
- **AND** 关闭后授权 MUST 继续 fail-closed，不得因本地资源不可用产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置、权限基线、角色值复制或 key schema 下沉到 `common`

### Requirement: 用户角色缓存配置与失效顺序门禁

系统 MUST 使用 `common/runtime/localcache` 的业务中立 cache-wide revision 与发布门禁保护 user-role cache。user-service MUST 私有拥有该 feature cache 的默认值、校验和配置映射。任何在失效前开始但未发布的旧回源结果 MUST NOT 在失效后写入缓存或返回给授权 caller；permission feature MUST NOT 维护重复的 generation 门禁，cache disabled 模式 MUST 直接回源并保持 fail-closed。

#### Scenario: 单用户失效抑制旧 load 返回与写回

- **WHEN** 用户角色 cache miss 已经开始为某个用户回源，且该用户的 Add、Remove 或 Replace 用户角色绑定成功后调用 `InvalidateUserRole`
- **THEN** resolver MUST 调用 `Invalidate(userID.String())`，并在方法返回时完成通用 revision 提升与指定 item 删除
- **AND** 失效前开始但失效后完成的旧回源结果 MUST NOT 写入该用户缓存或返回给当前授权 caller
- **AND** localcache 透明重试成功时后续授权 MUST 使用失效后的最终角色集合

#### Scenario: 全量失效抑制所有旧 load 返回与写回

- **WHEN** 一个或多个用户角色 cache miss 已经开始回源，且系统调用 `InvalidateAllUserRoles`
- **THEN** resolver MUST 调用 `InvalidateAll()`，并在方法返回时完成通用 revision 提升与全部 item 删除
- **AND** 全量失效前开始但失效后完成的任一旧回源结果 MUST NOT 写入缓存或返回给授权 caller
- **AND** localcache 透明重试成功时后续授权 MUST 使用全量失效后的最终角色集合

#### Scenario: 连续失效竞态保持 fail-closed

- **WHEN** `RolesForUser` 的首次回源结果因通用 revision 变化被抑制
- **THEN** localcache MUST 透明重试一次且 permission feature MUST NOT 增加 generation-aware retry wrapper
- **WHEN** 第二次回源仍被失效并返回 `ErrInvalidated`
- **THEN** 当前授权请求 MUST fail-closed，MUST NOT 使用旧角色集合产生允许结果
- **AND** loader 错误、context 取消或过期回源结果 MUST NOT 写入缓存

#### Scenario: cache disabled 模式保持直接回源

- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST NOT 创建通用 loading cache，并逐次从 PostgreSQL 回源当前启用角色
- **AND** `InvalidateUserRole` 与 `InvalidateAllUserRoles` MUST 保持同步幂等且不得引入旧 load 写回路径
- **AND** 回源成功 MUST 返回独立角色 ID slice，回源错误或 context 取消 MUST 保持 fail-closed

#### Scenario: User-role cache 默认值与创建

- **WHEN** `rbac.user_role_cache` 未配置
- **THEN** user-service MUST 使用 `enabled=true`、`size=100000`、`ttl=5s` 和 `load_timeout=500ms` 的完整默认值
- **WHEN** `rbac.user_role_cache.enabled=true`
- **THEN** `size`、`ttl` 和 `load_timeout` MUST 为正值
- **AND** permission feature MUST 通过集中转换创建具名 `rbac_user_roles` loading cache，配置的 `size` MUST 映射为最大 item 数

#### Scenario: User-role cache 禁用

- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 忽略 cache 的 `size`、`ttl` 和 `load_timeout`，不创建通用 loading cache，并逐次从 PostgreSQL 回源当前启用角色
- **AND** direct resolver MUST 返回独立角色 ID slice、记录 `LoadSuccess` 或 `LoadError`，并在回源错误或 context 取消时保持 fail-closed

#### Scenario: RBAC settings 依赖边界

- **WHEN** composition 构造用户角色 resolver、policy loader 或其他 RBAC runtime 资源
- **THEN** permission/RBAC provider MUST 接收只包含职责所需字段的 RBAC settings
- **AND** permission/RBAC feature MUST NOT 依赖完整 user-service 根配置或读取 auth、Ent、resources 等无关配置段
- **AND** feature cache 配置、必需缓存名和角色值复制语义 MUST 留在 user-service，不得进入 `common/runtime/localcache`

### Requirement: RBAC policy revision、同步与事务 outbox

系统 MUST 以 PostgreSQL 中追加式 policy revision 及事务 outbox 作为在线 RBAC 写入与跨副本恢复的权威事实。revision counter、业务 mutation、revision 与 outbox event MUST 在同一 transaction 中保持提交顺序与原子性；Redis Cluster key、channel 和 Pub/Sub MUST 只传播数据库 revision 并加速收敛。API MUST 以业务 transaction 是否提交表达写入结果，提交后的本地同步失败不得把已提交 mutation 返回为失败。

#### Scenario: 数据库 revision 发布与补偿

- **WHEN** 在线 RBAC 写操作提交新的数据库 policy revision
- **THEN** Redis version key MUST 位于稳定 hash tag 下，并允许 Redis Cluster client 写入该 revision
- **AND** adapter MUST NOT 使用 `INCR`、时间戳或本地计数器生成 revision，较小 revision MUST NOT 覆盖 Redis 中已存在的较大 revision
- **AND** 本地 reload、revision 发布和周期性 version check 的错误语义 MUST 保持 fail-closed、可恢复与可诊断

#### Scenario: Pub/Sub 通知与 watcher 生命周期

- **WHEN** watcher 订阅 policy refresh channel 或接收远端更新
- **THEN** channel 名称 MUST 使用稳定 hash tag 或 Cluster-compatible 命名
- **AND** watcher 停止、cache 关闭或 RBAC runtime 关闭 MUST NOT 关闭共享 Redis Cluster client

#### Scenario: 并发 mutation 不得按 revision 逆序提交

- **WHEN** 两个在线 RBAC mutation 并发执行且较早 mutation 已获得 revision N 但尚未提交
- **THEN** 后续 mutation MUST NOT 以 revision N+1 先行提交
- **AND** 后续 mutation MUST 等待前一事务提交或回滚后再获得可提交 revision

#### Scenario: 已有 counter 直接原子递增

- **WHEN** 固定 revision counter 行已经存在且在线 RBAC mutation 分配新 revision
- **THEN** transaction MUST 原子递增 counter 并使用返回值写入 revision 与 outbox
- **AND** 正常路径 MUST NOT 为初始化重复读取最大 revision 或重建 counter

#### Scenario: counter 缺失时从已有最大 revision 幂等初始化

- **WHEN** 固定 revision counter 行不存在且数据库已经存在零个或多个已提交 policy revision
- **THEN** 当前在线 mutation transaction MUST 读取最大已提交 revision，并通过 Ent 幂等创建与该值对齐的固定 counter 行
- **AND** transaction MUST 在初始化后原子递增 counter，使新 revision 大于全部已有 revision
- **AND** migration MUST NOT 依赖手写 seed `INSERT` SQL 创建 counter 行

#### Scenario: 并发首次初始化保持提交顺序

- **WHEN** 多个在线 RBAC mutation 并发发现固定 counter 行不存在
- **THEN** 固定主键冲突和 counter 行锁 MUST 串行化初始化与后续递增
- **AND** 每个成功 mutation MUST 获得唯一连续 revision，较大 revision MUST NOT 先于较小 revision 提交

#### Scenario: 初始化或事实写入失败完整回滚

- **WHEN** counter 初始化、递增、revision、outbox 或 transaction commit 任一步失败
- **THEN** 当前 transaction 的业务 mutation、counter 变化、revision 和 outbox MUST 全部回滚
- **AND** 后续重试 MUST 能重新执行缺失初始化或正常原子递增

#### Scenario: 较小 revision 通知晚到

- **WHEN** 实例已应用较大 revision 后收到较小 revision 的 `policy_changed` 通知
- **THEN** 实例 MUST 至少重新读取并应用一次当前 PostgreSQL 权威快照
- **AND** applied revision MUST NOT 倒退，旧候选 MUST NOT 覆盖较新候选

#### Scenario: 重复全局通知被合并

- **WHEN** 多条重复或乱序 `policy_changed` 通知并发触发刷新
- **THEN** engine MAY coalesce 同一时刻的刷新请求
- **AND** 所有调用完成时实际 enforcer MUST 对应不低于最高 target 的当前权威快照

#### Scenario: 强制刷新加入正在构造的普通 reload

- **WHEN** 强制刷新请求在普通 reload 已开始读取数据库后加入同一 flight
- **THEN** engine MUST 在该强制请求之后重新读取一次 PostgreSQL 快照
- **AND** MUST NOT 把强制请求到达前构造的候选视为该请求已经完成

#### Scenario: 漏收前序用户绑定通知后收到更高 revision

- **WHEN** 实例漏收用户 A 的 `user_role_changed` event，随后收到用户 B 对应的更高数据库 revision
- **THEN** watcher MUST 追赶到数据库 revision 并失效全部 user-role cache
- **AND** 用户 A 的旧缓存 MUST NOT 因后续消息只包含用户 B 而永久保留

#### Scenario: 提交后本地 reload 失败

- **WHEN** RBAC mutation、revision 和 outbox 已提交，但本地 policy reload 失败
- **THEN** API MUST 返回该 mutation 的成功结果
- **AND** 本实例授权 MUST fail-closed，pending outbox MUST 保持可投递并在后台恢复 projection

#### Scenario: 提交前任一步失败

- **WHEN** 业务 mutation、revision counter、revision、outbox 或 transaction commit 任一步失败
- **THEN** API MUST 返回失败并且 transaction 内全部变化 MUST 回滚
- **AND** command MUST NOT 执行提交后本地同步

#### Scenario: 绑定写响应不执行提交后查询

- **WHEN** 用户角色或角色权限 Add、Remove 或 Replace transaction 成功
- **THEN** store MUST 返回同一 transaction 内构造的最终绑定集合与 committed revision
- **AND** command MUST NOT 为构造成功响应在 commit 后重新查询数据库

#### Scenario: 在线 mutation 原子提交 revision 与 outbox

- **WHEN** 角色创建、角色更新、角色启停、角色权限添加/替换/删除或用户角色添加/替换/删除成功提交
- **THEN** 数据库 MUST 同时存在对应业务变更、一条已提交 policy revision 和一条引用该 revision 的 pending outbox event
- **AND** 不同成功 mutation 的 revision MUST 全局唯一且按数值单调递增
- **AND** revision 序列 MAY 因事务回滚存在空洞，调用方 MUST NOT 假设 revision 连续

#### Scenario: transaction 任一步失败时完整回滚

- **WHEN** 业务 mutation、revision 插入、outbox 插入或 transaction commit 任一步失败
- **THEN** 系统 MUST 返回错误并回滚该 transaction 内全部业务、revision 和 outbox 写入
- **AND** 系统 MUST NOT 执行本地 reload、缓存失效、Redis version 写入或 Pub/Sub 发布

#### Scenario: 校验失败不分配 revision

- **WHEN** 在线写请求因输入非法、对象不存在、对象不可用、系统角色保护、绑定冲突或其他业务校验失败而未产生 mutation
- **THEN** 系统 MUST NOT 创建 policy revision 或 outbox event
- **AND** 已有业务关系和授权投影 MUST 保持不变

#### Scenario: outbox event 契约

- **WHEN** 系统为已提交 revision 创建 outbox event
- **THEN** event MUST 包含稳定 event ID、唯一 revision、`kind`、`reason`、相关 `role_id`/`user_id`/`permission_id`、`status`、`attempt_count`、`next_attempt_at`、`last_error`、唯一幂等键、`created_at`、`updated_at` 和 `delivered_at`
- **AND** 新 event 的 `status` MUST 为 `pending`、`attempt_count` MUST 为零、`delivered_at` MUST 为空
- **AND** event kind MUST 区分全局 `policy_changed` 与定向 `user_role_changed`，幂等键 MUST 能由 revision 稳定确定

#### Scenario: 即时同步失败后保留恢复事实

- **WHEN** PostgreSQL transaction 已提交但本地 reload、缓存失效、Redis version 写入或 Pub/Sub 发布失败
- **THEN** 对应 revision 和 pending outbox event MUST 保持已提交且不得被删除、回滚或标记为已投递
- **AND** API MUST 返回已提交 mutation 的成功结果，可靠恢复 MUST NOT 依赖该次 Redis 操作成功

#### Scenario: 离线写入边界

- **WHEN** RBAC seed、bootstrap 或受控 migration 修改离线系统数据
- **THEN** 本 change MUST NOT 要求这些离线流程伪装成在线 outbox dispatcher 或宣称已完成副本同步
- **AND** 运维 MUST 继续通过显式 reload 或滚动重启使授权投影收敛

### Requirement: RBAC policy outbox 可靠投递

系统 MUST 以 PostgreSQL 中已提交的 RBAC policy outbox event 作为跨副本 revision 通知的可靠恢复事实，并由显式 dispatcher 对到期 event 执行 claim、Redis publish、成功 ack 和失败退避。user-service MUST 私有拥有轮询、批量、claim lease 与退避配置，并通过 permission lifecycle 启停同一 dispatcher 实例。dispatcher MUST 提供至少一次投递并在进程崩溃或 Redis 故障后自动恢复；Redis MUST 只作为可重放加速层。dispatcher 后台 goroutine 发生 panic 时 MUST fail-closed 停止运行，并记录包含 recovered value、稳定错误分类和 stack trace 的结构化日志。

#### Scenario: dispatcher 后台 panic recovery 可观测性

- **WHEN** dispatcher 后台 `run` 循环发生 panic
- **THEN** recovery 日志 MUST 记录 `error_category=unexpected_exit`
- **AND** recovery 日志 MUST 记录 `recovered` 字段，值来自 `recover()` 捕获结果
- **AND** recovery 日志 MUST 记录 stack trace 字段
- **AND** dispatcher MUST 将 `LastErrorCategory` 更新为 `unexpected_exit`
- **AND** dispatcher MUST 停止当前 ticker、标记 `Running=false`、上报 `DispatcherRunningObserved(false)` 并关闭当前 `done`
- **AND** 后续调用 `Stop(ctx)` MUST 幂等稳定返回
- **AND** dispatcher MUST NOT 因本场景自动重启后台 loop

### Requirement: RBAC revision 通知、幂等消费与故障验收

Redis policy refresh 消息 MUST 使用显式版本化 envelope 携带稳定 event identity、数据库 `policy_revision`、change kind、reason 及相关对象 ID。publisher 和 watcher MUST 接受重复与乱序，并通过可控故障验收证明 dispatcher、watcher、Casbin projection 与 user-role cache 最终收敛；Redis revision cache 与本地 tracker MUST 只按 max 推进。

#### Scenario: 发布完整 revision 通知

- **WHEN** dispatcher 发布 `policy_changed` 或 `user_role_changed` event
- **THEN** payload MUST 包含 `schema_version`、`event_id`、`idempotency_key`、`policy_revision`、`kind`、`reason` 和 publisher instance ID
- **AND** payload MUST 携带 event 中存在的 `user_id`、`role_id`、`permission_id`，缺失的可选 ID MUST 保持为空
- **AND** publisher MUST 以原子 max 语义缓存数据库 revision，较小或重复 revision MUST NOT 使 Redis 值倒退

#### Scenario: 重复与乱序通知保持幂等

- **WHEN** watcher 重复收到同一 event，或先收到较大 revision 后收到较小 revision
- **THEN** `policy_changed` MUST 安全地从当前 PostgreSQL 权威投影执行全量 reload，`user_role_changed` MUST 安全地失效消息指定用户的角色缓存
- **AND** watcher MUST NOT 仅因消息 revision 不大于本地已知最大值而跳过该消息要求的缓存失效或 reload 副作用
- **AND** 完成副作用后本地 tracker MUST 只按 max 推进，MUST NOT 回退已知 revision

#### Scenario: 非法或旧协议消息被拒绝

- **WHEN** payload 缺少必需字段、包含未知 schema version/kind 或非法 UUID
- **THEN** watcher MUST 拒绝执行该消息并记录不含完整 payload 或敏感数据的诊断错误
- **AND** watcher MUST NOT 尝试按旧消息形状解析，也 MUST NOT 回退到 Redis counter 语义

#### Scenario: Redis 不是可靠或权威存储

- **WHEN** Redis revision cache 更新成功但 Pub/Sub publish 失败，或 Pub/Sub 消息丢失
- **THEN** outbox event MUST 保持未完成并可重试，watcher 的周期补偿 MAY 使用 Redis 已知最大 revision 加速发现变化
- **AND** PostgreSQL revision、outbox event 与 RBAC 关系投影 MUST 继续是恢复和授权数据的权威来源
- **AND** 系统 MUST NOT 要求 Redis publish 与 PostgreSQL mutation transaction 原子化

#### Scenario: Redis 故障恢复后副本无需新写即可收敛

- **WHEN** 在线 RBAC 写入已成功提交数据库 revision，但 Redis version 发布或 Pub/Sub 通知在故障注入下失败，随后 Redis 恢复且没有新的 RBAC 写入
- **THEN** 故障注入测试 MUST 验证 watcher 或周期性版本补偿最终使所有参与副本的 lag 归零
- **AND** 每个副本的 applied revision MUST 收敛到数据库最新 revision
- **AND** 每个副本的 Casbin projection 和用户角色 cache 解析结果 MUST 与数据库权威关系一致

#### Scenario: reload 逆序完成时最新 revision 保持权威

- **WHEN** 两次 RBAC policy reload 被故障注入控制为后发 revision 先完成、先发 revision 后完成
- **THEN** 故障注入测试 MUST 验证最终 applied revision 仍为最新 revision
- **AND** 旧 revision 的 reload 结果 MUST NOT 覆盖较新的 Casbin projection 或用户角色 cache 状态
- **AND** 授权 allow/deny 结果 MUST 与最新数据库关系一致

#### Scenario: Add Remove Replace 重放保持幂等收敛

- **WHEN** 角色权限或用户角色绑定的 Add、Remove、Replace 同步事件被故障注入为重复投递、乱序投递或 dispatcher 重试
- **THEN** 故障注入测试 MUST 验证通知不丢失且重放不会产生非幂等破坏
- **AND** 最终数据库 revision、applied revision、Casbin projection 和用户角色 cache MUST 收敛到最后一次成功提交的数据库状态

#### Scenario: 100 并发 RBAC 写入最终收敛

- **WHEN** 测试并发执行 100 个 RBAC 写操作，并注入 loader 阻塞、watcher 消息乱序或 cache loader 延迟
- **THEN** 故障注入测试 MUST 验证所有成功提交写入对应的最终数据库 revision 可被观察到
- **AND** 所有参与副本的 applied revision MUST 最终等于最新数据库 revision
- **AND** 抽样或完整授权断言 MUST 证明 Casbin projection 和用户角色 cache 与最终数据库关系一致

#### Scenario: 测试说明记录风险与收敛条件

- **WHEN** 新增或更新 RBAC policy sync 故障注入测试
- **THEN** `docs/TESTING.md` 或相关测试说明 MUST 记录每个故障注入场景对应的风险、预期收敛条件和运行方式
- **AND** 文档 MUST 明确完整真实 PostgreSQL/Redis 容器门禁通过根 `make test-containers` 运行，窄化调试通过显式 `-args -aegiscore.testcontainers` 启用

### Requirement: RBAC watcher 以数据库 revision 补偿收敛

RBAC watcher MUST 以 PostgreSQL latest policy revision作为副本补偿和reload目标的权威来源，并以本地Casbin engine实际应用的projection revision作为本地状态。Redis Pub/Sub及其payload revision MUST 只作为可丢失、可重复、可乱序的唤醒hint，Redis counter、key缺失或重建状态 MUST NOT 决定副本已经收敛。授权热路径 MUST NOT 因本要求增加PostgreSQL或Redis revision读取。

#### Scenario: Pub/Sub消息触发数据库revision校准

- **WHEN** watcher收到合法policy refresh消息
- **THEN** watcher MUST 读取数据库latest policy revision并以该revision作为`ReloadToRevision`或等价revision-aware reload的目标
- **AND** payload revision MUST 只作为hint和低风险诊断字段，MUST NOT 直接推进local applied projection revision、清零lag或覆盖数据库latest revision
- **AND** payload重复、乱序或低于local applied revision时，engine投影 MUST 保持不倒退，消息要求的既有cache side effect仍 MUST 保持幂等语义

#### Scenario: Pub/Sub丢失后的周期补偿

- **WHEN** 数据库latest policy revision高于local applied projection revision且对应Pub/Sub消息丢失
- **THEN** 周期性`CheckVersion`或等价补偿检查 MUST 直接读取数据库latest revision并触发revision-aware reload
- **AND** watcher MUST 在后续成功检查与reload中最终使local applied projection revision不低于数据库latest revision
- **AND** 补偿判断 MUST NOT 依赖Redis counter存在、领先或与数据库latest相等

#### Scenario: Redis状态不影响数据库补偿

- **WHEN** Redis counter不存在、落后于数据库latest、被重建为较小值或Redis从故障中恢复
- **THEN** watcher MUST 继续以数据库latest revision判断是否需要reload
- **AND** 系统 MUST NOT 因Redis值等于或低于local applied revision而跳过数据库revision超前所需的补偿
- **AND** Redis恢复后收到的旧消息 MUST NOT 使旧revision覆盖新projection或降低local applied revision

#### Scenario: 数据库revision source不可用

- **WHEN** Pub/Sub唤醒或周期检查无法读取数据库latest policy revision
- **THEN** watcher MUST 记录稳定的revision store unavailable诊断并保留底层cause用于日志
- **AND** watcher MUST NOT 使用Redis counter或payload revision冒充数据库目标、记录reload success或把lag重置为`0`
- **AND** 后续数据库读取恢复时，周期检查或下一条hint MUST 重新校准latest revision并继续补偿

#### Scenario: reload失败后恢复

- **WHEN** 数据库latest revision高于local applied revision但本地reload失败、被取消或未达到目标
- **THEN** engine MUST 保留上一成功projection及其applied revision并保持fail-closed健康语义，watcher MUST 记录reload failure且不得宣称收敛
- **AND** 后续Pub/Sub hint或周期检查 MUST 再次读取数据库latest revision并重试
- **WHEN** 后续reload成功且实际applied revision不低于读取到的database latest revision
- **THEN** watcher MUST 记录reload success并恢复收敛状态

#### Scenario: revision查询依赖边界

- **WHEN** permission feature查询latest policy revision
- **THEN** application MUST 拥有只读最小revision source port，PostgreSQL/Ent adapter MUST 留在permission infrastructure，named database与lifecycle选择 MUST 留在composition
- **AND** revision查询语义 MUST NOT 下沉到`common/`、`internal/shared/`或`internal/integration/`，application/domain MUST NOT 导入Ent concrete client或predicate包
- **AND** watcher MUST 复用现有 policy revision source、outbox schema 与 dispatcher，MUST NOT 创建第二套 revision 或投递机制

### Requirement: RBAC 生产源码与测试辅助代码边界

RBAC 生产源码 MUST 只保留生产运行、生成期框架入口或稳定服务 API 真实消费的实现。仅为测试 fake、测试数据组装或未来示例存在的构造器、helper、alias、wrapper 和注释伪代码 MUST 位于测试编译单元或被删除；测试 MUST 直接验证现有生产入口和能力所有者，不得为测试便利扩大生产 API。

#### Scenario: watcher 测试使用 fake 依赖

- **WHEN** permission Redis watcher 测试需要注入 `policySubscriptionStore` fake 或测试 metrics
- **THEN** 测试专用构造 MUST 位于 `_test.go` 并复用生产内部核心构造
- **AND** 生产源码 MUST 只保留真实运行路径消费的 `NewWatcher` 及其内部实现，MUST NOT 保留测试专用转发构造器或扩大 `WatcherParams` 依赖类型

#### Scenario: permission transport 验证显式授权白名单

- **WHEN** permission HTTP transport 测试验证显式授权白名单绕过语义
- **THEN** transport MUST 直接使用 `common/http/middleware` 拥有的 Casbin whitelist rule 和 option
- **AND** permission feature MUST NOT 保留没有服务专用语义的 type alias、转发 wrapper、兼容名称或虚构生产消费者

#### Scenario: 默认角色 catalog 只描述当前基线

- **WHEN** `internal/shared/rbacbaseline` 定义当前默认系统角色及权限绑定
- **THEN** 生产 catalog MUST 只包含当前真实角色和绑定所需代码
- **AND** 系统 MUST NOT 为尚未存在的默认角色保留未消费 helper、注释掉的 role block、示例 ID 或兼容分支

#### Scenario: 生产调用图静态检查

- **WHEN** CI 或开发者对 user-service 运行不包含测试入口的生产调用图 deadcode 检查
- **THEN** RBAC 手写生产代码 MUST NOT 报告仅由测试引用的 watcher 构造器、baseline helper 或 authorization wrapper
- **AND** Ent schema/mixin 生成期入口和其他明确归属 capability 的测试支持入口 MUST 单独复核，不得通过删除共享公开 API 或生成期入口消除报告

### Requirement: Casbin reload metrics 归属 permission feature

Casbin policy reload recorder MUST 由 user-service permission/RBAC 边界拥有。permission feature MUST 定义 Engine 消费的最小 reload recorder interface、disabled metrics 时使用的非 nil no-op 实现，以及使用通用 metrics Provider 注册 Prometheus collector 的实现。该 recorder MUST NOT 位于 `common/runtime/observability/metrics`、`user-service/internal/shared` 或 role feature。

#### Scenario: Engine 使用 permission-owned recorder

- **WHEN** permission composition 构造 Casbin Engine
- **THEN** Engine MUST 接收 permission-owned reload recorder interface
- **AND** Engine MUST NOT import `common/runtime/observability/metrics` 以获得 Casbin reload 业务接口
- **AND** Engine 的授权、reload、health 和 initialization 投影 MUST 继续指向同一个 engine 实例

#### Scenario: 指标名称和语义保持不变

- **WHEN** policy reload 成功或失败
- **THEN** recorder MUST 分别增加 `aegiscore_casbin_policy_reloads_total{status="success"}` 或 `aegiscore_casbin_policy_reloads_total{status="failure"}`
- **AND** recorder MUST 使用 `aegiscore_casbin_policy_reload_last_success` 以 `1` 表示最近 reload 成功，以 `0` 表示最近 reload 失败
- **AND** 指标 MUST NOT 增加用户、角色、权限、revision、Redis key、SQL、原始错误或其他高基数 label

#### Scenario: metrics 禁用时使用安全空实现

- **WHEN** 全局 metrics provider 缺失或禁用
- **THEN** permission feature MUST 为 Casbin Engine 注入非 nil no-op reload recorder
- **AND** no-op recorder MUST 保持 reload、watcher、initializer 和授权 fail-closed 行为不变，不得注册 collector 或引入 nil 分支

#### Scenario: 不改变 policy reload 行为

- **WHEN** Casbin policy 初始加载、显式 reload、Pub/Sub 触发 reload 或周期补偿 reload 执行
- **THEN** 本 change MUST NOT 改变 revision-aware loading、enforcer swap、防倒退、最近错误、readiness/startup 或 watcher 收敛语义
- **AND** 观测失败 MUST NOT 改变 reload 成功或失败的业务结果

### Requirement: RBAC policy sync 并发与状态测试门禁

系统 MUST 为 RBAC watcher 与 Casbin enforcer 的并发同步、补偿、关闭和 cancellation 语义提供 race/stress 测试门禁。测试 MUST 使用 deterministic fake、可控 channel 或同步 primitive 构造竞态，MUST 能在 `go test -race` 下稳定运行，并且 MUST NOT 依赖真实外部 Redis 或 PostgreSQL 服务。

#### Scenario: watcher 并发通知与周期补偿收敛

- **WHEN** watcher 并发接收多条重复、乱序或较小的 Pub/Sub policy hint，并同时触发周期性 PostgreSQL revision check
- **THEN** 测试 MUST 断言 watcher 只把数据库可见 revision 作为 reload target，并最终通过 revision-aware port 追赶到不低于最高权威 revision 的投影
- **AND** 测试 MUST 断言重复或旧 hint 不会导致 Casbin applied revision 倒退
- **AND** 定向 user-role cache invalidation 的副作用 MUST 按协议执行，但不得独立推进 Casbin applied revision

#### Scenario: watcher 重订阅与状态语义

- **WHEN** 已确认的 Redis Pub/Sub 订阅断连、message channel 关闭或 Receive 返回可恢复错误
- **THEN** 测试 MUST 断言 watcher 根生命周期仍保持 running，subscription state 进入 reconnecting 或等价的重订阅状态
- **AND** 周期性 PostgreSQL revision check MUST 在订阅退避期间继续运行
- **WHEN** 重订阅确认成功
- **THEN** 测试 MUST 断言 subscription state 恢复 connected，并清除当前 subscription 错误

#### Scenario: watcher Stop 竞态与取消语义

- **WHEN** watcher `Stop(ctx)` 与阻塞 revision source、阻塞 reload engine、订阅确认、Receive、退避 timer 或 payload delivery 并发发生
- **THEN** 测试 MUST 断言 Stop 在调用方 context 期限内取消内部 root context 并等待 watcher goroutine 退出，除非测试显式覆盖 Stop 超时语义
- **AND** Stop 完成后 watcher lifecycle MUST 为 stopped，subscription state MUST 为 stopped 或等价关闭状态
- **AND** 正常停止导致的 reconcile cancellation MUST NOT 记录为业务 failure、最近失败时间或当前 reconcile 错误
- **AND** Stop 超时 MUST 返回 context 错误，并保持后续重复 Stop 调用安全

#### Scenario: enforcer 多 waiter 与 reload coalescing

- **WHEN** 多个 goroutine 并发调用 `ReloadToRevision`、`RefreshToRevision` 或等价 revision-aware reload 入口，且 target revision 重复、乱序或递增
- **THEN** 测试 MUST 断言实际 reload 工作被串行化或 coalesce，最终 applied revision 不低于所有未取消等待方请求的最高 target
- **AND** 每个未取消等待方 MUST 只在 engine 实际 applied revision 不低于其 target 后返回成功
- **AND** 单个等待方 context cancellation MUST NOT 取消其他等待方仍需要的共享 reload

#### Scenario: enforcer root cancel、leader cancel 与强制刷新

- **WHEN** engine root context 被取消、reload leader context 被取消或 loader/reload gate 被阻塞
- **THEN** 测试 MUST 断言未完成等待方返回取消错误或对应 reload 错误，engine 不提升 applied revision，不清除最近失败状态，也不使用旧投影放行请求
- **WHEN** force refresh 请求在普通 reload 已经开始读取数据库后加入同一 flight
- **THEN** 测试 MUST 断言 engine 在 force refresh 到达后重新读取一次 PostgreSQL 快照，并且不得把 force 请求到达前构造的候选视为该请求已完成

### Requirement: RBAC policy sync 统一生命周期上下文

RBAC policy sync 的 dispatcher、watcher、subscriber 和 enforcer reload engine MUST 由 permission runtime 接收的显式服务 lifecycle root context 统一约束。后台运行 context MUST 从该 root context 派生；启动路径 MUST NOT 使用 `context.Background()` 建立独立 root，MUST NOT 保留无参 `Start()` 或等价兼容 adapter。`Stop(ctx)` 的 ctx MUST 只限制等待退出的期限，单个 reload waiter 的 ctx MUST 只取消该 waiter，MUST NOT 替代或取消仍被其他参与者需要的共享运行 root 或 reload flight。

#### Scenario: permission lifecycle 启动与停止后台同步链路

- **WHEN** permission runtime 启动、停止或执行启动失败回滚
- **THEN** lifecycle MUST 使用同一服务 lifecycle root context 显式启动 watcher、subscriber 和 dispatcher，并使 enforcer reload engine 受该 root cancellation 约束
- **AND** constructor MUST NOT 提前启动 goroutine，启动失败 MUST 幂等停止已启动资源
- **AND** 停止顺序 MUST 阻止新的 dispatch 或 reconcile 工作进入，再取消运行 root 并等待 in-flight 工作退出
- **AND** 任一组件 MUST NOT 关闭共享 PostgreSQL、Ent 或 Redis client

#### Scenario: lifecycle root cancellation 终止共享 reload flight

- **WHEN** 服务 lifecycle root context 被取消且 enforcer 仍有进行中的 reload flight 或等待方
- **THEN** reload engine MUST 取消未完成工作并使等待方返回 cancellation 或对应 reload error
- **AND** engine MUST NOT 提升 applied revision、清除最近失败状态或把未完成候选投影发布为成功结果
- **WHEN** 仅一个 reload waiter 的 context 被取消且其他 waiter 仍需要同一 flight
- **THEN** engine MUST 只结束该 waiter 的等待，共享 flight MUST 继续服务其他未取消 waiter

### Requirement: RBAC dispatcher batch partial success 与异常终态

RBAC policy outbox dispatcher 单次 batch MUST 返回结构化结果或等价结构化状态，使调用方能够区分 claim、publish、ack、failure record、claim lost、backlog/status refresh 和 context cancellation。dispatcher MUST 保留 partial success：单条事件失败 MUST NOT 阻断同 batch 后续未取消事件，已成功 publish 或 ack 的结果 MUST NOT 因最终返回 error 而被抹除。旧 error-only `DispatchOnce` 语义 MUST NOT 作为兼容行为保留。后台 loop panic MUST fail-closed 进入 `unexpected_exit`，并留下完整恢复证据。

#### Scenario: dispatcher batch 部分成功

- **WHEN** dispatcher claim 多个 due event，且其中部分事件发生 publish、ack、failure record 或 claim lost 错误
- **THEN** dispatcher MUST 继续处理同 batch 后续未取消 claim
- **AND** 结构化结果 MUST 暴露 claimed、delivered、acknowledged、retried、failed 和 status refresh 成功与否或等价信息
- **AND** 返回错误 MUST 可判别每个失败阶段，MUST NOT 暗示整个 batch 未发生成功投递
- **AND** 已成功 ack 的事件 MUST 保持 delivered，失败或失去 claim 的事件 MUST 按既有 lease recovery 与退避语义恢复

#### Scenario: claim、status refresh 与 cancellation 相互独立

- **WHEN** batch claim 失败
- **THEN** 结果 MUST NOT 伪造已 claim、已投递或已 ack 事件，并 MUST 标识 claim 阶段错误
- **WHEN** backlog/status refresh 失败但 batch 内已有事件成功投递
- **THEN** 结果 MUST 保留已成功事件计数，并独立标识 status refresh 失败
- **WHEN** context 在某个 claim 完成前被取消
- **THEN** dispatcher MUST 停止开始新的工作，且 MUST NOT 主动 Ack 或 Fail 当前未完成 claim，后续恢复 MUST 继续依赖 claim lease

#### Scenario: dispatcher 后台 panic recovery 可观测性

- **WHEN** dispatcher 后台 run loop 发生 panic
- **THEN** recovery 日志 MUST 记录 `error_category=unexpected_exit`
- **AND** recovery 日志 MUST 记录来自 `recover()` 的 recovered value 和 stack trace
- **AND** dispatcher MUST 将最近错误分类更新为 `unexpected_exit`，停止 ticker，设置 running=false，上报对应运行指标并关闭当前 done signal
- **AND** dispatcher MUST NOT 自动重启后台 loop，后续 `Stop(ctx)` MUST 幂等稳定返回

### Requirement: RBAC watcher 断连恢复与 final state

RBAC watcher MUST 分别维护 lifecycle、subscription 与 reconcile 状态，并以 PostgreSQL policy revision 作为最终权威事实。Redis subscription 断连、message channel 关闭或可恢复 Receive error MUST 触发重订阅而不是终止 watcher root lifecycle；订阅退避期间周期 reconcile MUST 继续运行。正常停止、Stop 等待超时、reconcile cancellation 与异常退出 MUST 形成可判别 final state，历史瞬时错误恢复后 MUST NOT 使 watcher 永久保持失败。

#### Scenario: Redis 断连与重订阅

- **WHEN** 已确认的 Redis subscription 断连、message channel 关闭或 Receive 返回可恢复错误
- **THEN** watcher lifecycle MUST 保持 running，subscription MUST 进入 reconnecting 或等价状态
- **AND** PostgreSQL revision reconcile MUST 在重订阅退避期间继续运行，Redis hint MUST NOT 取代数据库权威 revision
- **WHEN** subscriber 完成新的订阅确认
- **THEN** subscription MUST 恢复 connected，并清除当前 subscription error 与对应失败时间

#### Scenario: watcher 正常停止与 reconcile cancellation

- **WHEN** watcher root context 因正常 lifecycle shutdown 被取消，且 revision query、reload、订阅确认、Receive、退避 timer 或 payload handling 正在执行
- **THEN** watcher MUST 停止接收新工作并等待 in-flight 工作退出
- **AND** 由该正常停止直接导致的 reconcile cancellation MUST NOT 记录为业务 failure、最近失败时间或当前 reconcile error
- **AND** 后台 loop 真正退出后 lifecycle 与 subscription MUST 进入 stopped 或等价关闭终态

#### Scenario: watcher Stop 超时

- **WHEN** `Stop(ctx)` 的等待期限先于 watcher 后台 loop 退出到期
- **THEN** Stop MUST 返回调用方 context error，并保持内部 root cancellation 已发出
- **AND** watcher MUST NOT 在后台 loop 实际退出前伪造 stopped final state
- **AND** 后续重复 Stop MUST 保持安全，并可在后台退出后观察到 stopped final state

#### Scenario: subscriber 与 watcher 责任边界

- **WHEN** Redis 订阅需要建立、确认、取消、断连检测或重新建立
- **THEN** `common/runtime/redispubsub` subscriber MAY 提供无业务语义 lifecycle primitive
- **AND** policy revision envelope、数据库权威校准、reconcile 状态、user-role cache invalidation 和 watcher final state MUST 由 permission feature 拥有

### Requirement: RBAC policy sync race 与 stress 验证门禁

系统 MUST 为 dispatcher、watcher、subscriber 和 enforcer reload engine 的并发、关闭、异常与 cancellation 语义提供 race/stress 验证。测试 MUST 使用 deterministic fake、可控 channel、barrier 或等价同步 primitive，MUST NOT 依赖真实 Redis 或 PostgreSQL，并 MUST 断言规格状态而非偶然 goroutine 调度顺序。

#### Scenario: dispatcher、watcher 与 subscriber 并发验证

- **WHEN** 测试并发触发 dispatcher partial success、panic finalization、watcher 断连重订阅、reconcile、Stop 和 subscriber cancellation
- **THEN** 测试 MUST 在 `go test -race` 下稳定通过且不得报告 data race、goroutine leak 或重复关闭 panic
- **AND** 测试 MUST 覆盖 running、reconnecting、connected、stopped、unexpected_exit 和 Stop timeout 等适用状态迁移

#### Scenario: enforcer 多 waiter 与 force refresh 验证

- **WHEN** 多个 goroutine 以重复、乱序或递增 target revision 并发请求普通 reload 或 force refresh
- **THEN** 实际 reload MUST 串行化或 coalesce，未取消 waiter MUST 只在 applied revision 不低于各自 target 后成功
- **AND** 单个 waiter cancellation MUST NOT 取消其他 waiter 所需 flight
- **AND** force refresh 在普通 reload 已开始读取数据库后加入时，engine MUST 为 force 请求重新读取一次 PostgreSQL 快照

#### Scenario: 推荐验证命令

- **WHEN** 维护者验证 RBAC policy sync 的统一并发语义
- **THEN** SHOULD 运行 `go test -race -count=20 ./user-service/internal/features/permission/application ./user-service/internal/features/permission/infrastructure/redis ./user-service/internal/features/permission/infrastructure/casbin`
- **AND** SHOULD 再运行相关包普通测试、`openspec validate document-rbac-policy-sync-semantics --strict` 和 `make user-service-architecture-lint`

### Requirement: RBAC policy 写事务不得受共享 helper 隐式 5 秒断点限制

RBAC 角色、角色权限、用户角色、系统绑定、超级管理员 bootstrap 和 policy outbox claim 的 PostgreSQL 事务 MUST 依赖 `common/runtime/datastore` 修正后的事务 lifecycle 语义。无原始 deadline 时，这些事务 MUST NOT 因 datastore helper 的固定 5 秒 timeout 被自动回滚；事务耗时、锁等待和慢查询边界 MUST 由调用方显式 deadline 或数据库 timeout 策略控制。

#### Scenario: policy revision counter 锁等待超过 5 秒仍按策略完成
- **WHEN** RBAC policy 写事务在分配单调 revision 时等待固定 `rbac_policy_revision_counters` 行锁超过 `DefaultTransactionCleanupTimeout`
- **THEN** datastore helper MUST NOT 因隐藏 5 秒 deadline 取消该事务
- **AND** 锁释放后事务 MUST 能继续追加 policy revision 和 outbox event，并在原始 context 和数据库策略允许时提交成功

#### Scenario: 提交前请求取消仍不得提交 RBAC 写结果
- **WHEN** RBAC policy 写事务已完成业务 mutation、revision 分配和 outbox event 构造，但原始 request context 在 commit 前取消或超时
- **THEN** store MUST 拒绝提交该事务
- **AND** 系统 MUST 回滚角色、角色权限、用户角色、policy revision counter、policy revision 和 outbox event 变更
- **AND** application MUST NOT 发送 policy change 通知或触发本实例 reload

#### Scenario: RBAC 高并发写入不存在 helper 层固定 5 秒失败点
- **WHEN** 多个 RBAC 写请求并发竞争 revision counter、角色行锁或系统绑定事务资源
- **THEN** 请求成功、失败或超时 MUST 由原始 context deadline、数据库错误或显式数据库 timeout 决定
- **AND** 系统 MUST NOT 因 `DefaultTransactionCleanupTimeout` 被传入 `BeginTx` 而在约 5 秒处形成固定自动回滚断点

#### Scenario: RBAC 本地代码不保留兼容绕过分支
- **WHEN** RBAC PostgreSQL store 使用事务 helper 执行在线写入、seed/system binding、bootstrap 或 outbox claim
- **THEN** store MUST 直接消费修正后的 `datastore.BeginTransaction` 行为
- **AND** store MUST NOT 增加旧 5 秒行为兼容开关、绕过参数或重复事务 lifecycle helper

