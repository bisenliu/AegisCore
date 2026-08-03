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

系统 MUST 提供角色创建、更新、启停、详情、列表和角色权限绑定，以及用户角色绑定的查询、添加、移除和完整替换能力。公开写接口 MUST NOT 允许创建或篡改系统角色；绑定 MUST 只引用存在的代码基线权限、未软删除用户和启用角色，完整替换 MUST 保持事务性。

#### Scenario: 角色目录写入和查询

- **WHEN** 授权调用方提交合法角色信息和存在的权限集合
- **THEN** 系统 MUST 创建非系统角色并写入角色权限绑定，成功响应 MUST 返回新建角色实体
- **WHEN** 授权调用方更新、启用或停用存在的普通角色
- **THEN** 系统 MUST 持久化变更，成功响应 MUST NOT 包含更新后的角色实体
- **WHEN** 授权调用方查询角色详情或分页查询角色
- **THEN** 系统 MUST 返回角色数据、权限摘要和共享 pagination 信息

#### Scenario: 系统角色与权限绑定保护

- **WHEN** 普通角色接口尝试创建、修改或停用系统角色
- **THEN** 系统 MUST 拒绝操作并保持系统角色及其基线语义不变
- **WHEN** 角色权限写请求引用不存在或不属于当前代码基线投影的权限
- **THEN** 系统 MUST 拒绝写入并保持已有关系不变
- **AND** role application MUST 通过 permission application 拥有的最小查询端口校验权限，MUST NOT 导入 permission infrastructure
- **WHEN** 调用方把任意现存基线权限绑定给普通角色
- **THEN** 系统 MUST 允许绑定且 MUST NOT 检查权限 active 或 system 状态
- **AND** Permission 状态语义的移除 MUST NOT 删除或改变 `Role.Active` 与 `Role.IsSystem`

#### Scenario: 角色权限替换、停用与 seed

- **WHEN** 调用方以合法权限集合完整替换角色权限
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定，任一非幂等错误 MUST 回滚全部变更
- **WHEN** 角色被停用
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

系统 MUST 在认证后使用 RBAC 中间件保护权限、角色和用户业务接口，并以 PostgreSQL 关系数据与单调 policy revision 作为业务权威来源，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。每个本地 Casbin enforcer MUST 与其实际加载的数据库 policy revision 绑定，applied revision MUST 表示该 engine 当前成功应用的授权投影，MUST NOT 表示 Redis 通知序号、消息接收进度或 reload attempt。授权 MUST 使用稳定 subject、Gin route template 和 HTTP method，并在任何身份、策略、revision 或执行异常下 fail-closed。Redis 与 Pub/Sub MUST 只传播数据库 revision并加速副本收敛；授权热路径 MUST 使用本地投影，MUST NOT 每请求读取 Redis或PostgreSQL revision。

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
- **WHEN** 用户角色回源失败、context 取消、Casbin 执行错误、policy 未加载、目标 revision 未追平或最近一次 reload 失败
- **THEN** 系统 MUST 拒绝请求并暴露 policy 不可用 readiness/startup 状态，MUST NOT 使用保留的旧 enforcer 继续允许请求
- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS`
- **THEN** 中间件 MUST 允许请求并 MUST NOT 调用授权服务
- **WHEN** 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 中间件链；token version validator、RBAC authorizer 或必需 route registrar 缺失时系统 MUST 拒绝注册部分路由

#### Scenario: revision-aware policy 快照加载

- **WHEN** policy loader 面向目标数据库 revision 构造授权策略
- **THEN** loader MUST 在同一 PostgreSQL 一致性快照中读取可见 latest policy revision 与启用角色、角色权限绑定和 permissions 投影，并返回 `PolicySet{Revision, PermissionRules}` 或等价结构
- **AND** 返回的 revision MUST 不低于目标 revision，规则 MUST 与该 revision 所属数据库快照绑定；loader MUST NOT 为旧规则附加较新的 revision
- **AND** 用户身份解析 MUST 排除已软删除用户，loader MUST NOT 使用权限 active predicate，独立 `casbin_rules` 表 MUST NOT 成为业务权威来源
- **WHEN** 当前快照可见 revision 低于目标 revision
- **THEN** loader MUST 在 context 期限内结束旧快照并使用新快照重试，MUST NOT 返回低于目标的 policy、在旧快照内无限等待或将通知 revision直接作为快照 revision
- **WHEN** target revision 为 0且数据库尚无 policy revision记录
- **THEN** loader MUST 以revision 0加载当前基线投影，并保持超级管理员wildcard policy语义

#### Scenario: revision-aware engine 交换与防倒退

- **WHEN** engine 收到目标 revision 并完成候选 `PolicySet` 与 enforcer 构造
- **THEN** engine MUST 在同一锁定临界区比较候选 revision与当前 applied revision，并原子交换 enforcer、applied revision与成功状态
- **AND** 只有更高候选 revision可以替换当前enforcer，相等候选 MUST 幂等成功，较低候选 MUST 被丢弃且不得覆盖或降低当前投影
- **WHEN** revision 1的reload在revision 2成功应用后才完成
- **THEN** 最终enforcer和applied revision MUST 仍对应revision 2或更高的数据库快照
- **AND** engine、tracker/status、metrics与health暴露的applied revision MUST 来自同一实际投影状态，MUST NOT由watcher独立推进

#### Scenario: 同实例并发 reload 收敛

- **WHEN** 同一实例并发收到多个数据库target revision
- **THEN** engine MUST 串行化或coalesce实际reload工作，将pending target单调提升到已观察到的最大值，并防止并发构造导致投影倒退
- **AND** 等待方只有在实际applied revision不低于其target时才能观察到成功；单个等待方context取消 MUST NOT取消其他调用仍需要的共享reload
- **WHEN** 100个并发policy写入触发reload且数据库latest revision可见
- **THEN** reload稳定后engine applied revision MUST 等于加载时数据库latest revision且不低于全部target中的最大值
- **AND** 系统 MUST NOT要求revision连续或为每个中间revision分别构造enforcer

#### Scenario: 初始加载、reload 失败与恢复

- **WHEN** user-service启动permission/RBAC模块
- **THEN** composition层 MUST使用可取消或带超时的启动context显式加载当前数据库latest policy revision
- **WHEN** 初始加载失败、被取消或不能达到目标revision
- **THEN** engine MUST保留实际applied revision、记录最近错误和reload失败指标，后续授权 MUST fail-closed，`app.Start` MUST保持成功
- **AND** reload状态和readiness/startup MUST保留失败信息并拒绝接入业务流量
- **WHEN** 已存在成功投影后的reload加载、构造或交换失败
- **THEN** engine MUST保留上一成功enforcer及其applied revision，MUST NOT提升revision、清除失败或使用旧投影放行请求
- **WHEN** 后续显式reload、Pub/Sub或周期补偿成功应用不低于目标的数据库快照
- **THEN** engine MUST原子替换或确认当前投影、清除最近reload错误并恢复readiness/startup

#### Scenario: 用户角色缓存键、容量与值隔离

- **WHEN** user-role cache 启用
- **THEN** permission feature MUST 使用 `uuid.UUID` 作为真实业务 key，并将配置的正数 size 映射为最大 item 数
- **AND** common MUST NOT 字符串化 UUID、接收 key encoder 或暴露底层 cache option
- **WHEN** 缓存命中
- **THEN** loader 写入缓存前和 `RolesForUser` 返回前 MUST 复制 `[]uuid.UUID`，调用方修改返回 slice MUST NOT 污染缓存或后续读取
- **AND** `common/runtime/localcache` MUST NOT 承担角色 ID clone 语义

#### Scenario: 用户角色回源与缓存关闭

- **WHEN** 缓存未命中
- **THEN** 系统 MUST 合并同一用户的并发回源并查询 PostgreSQL 中的当前启用角色，loader 错误 MUST NOT 写入缓存
- **WHEN** cache 已关闭或回源失败
- **THEN** 授权 MUST fail-closed，MUST NOT 因 cache 不可用产生允许结果
- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice并保持fail-closed；direct stats source MUST使用`LoadSuccess`与`LoadError`表达逐次结果

#### Scenario: 在线写后同步与数据库 revision 目标

- **WHEN** 角色状态、角色权限或用户角色绑定通过在线API与policy revision原子提交成功
- **THEN** 本实例coordinator MUST使用该数据库revision作为reload或cache invalidation目标，outbox dispatcher MUST传播同一数据库revision
- **AND** reload、cache invalidation或通知失败 MUST保持可诊断和fail-closed语义，MUST NOT把通知接收、Redis max写入或publish成功标记为engine已应用
- **AND** `PolicyChangeNotifier` MUST是正式command service的必需依赖并接收数据库revision
- **WHEN** 权限投影由离线migration、seed或bootstrap改变
- **THEN** 离线命令 MUST NOT宣称已完成在线policy refresh，运维 MUST显式创建/传播对应revision、执行revision-aware reload或滚动重启副本

#### Scenario: watcher、重复通知与副本收敛

- **WHEN** watcher通过Pub/Sub或周期性检查发现数据库policy revision高于engine applied revision
- **THEN** watcher MUST以该revision调用revision-aware application port，只有engine成功应用不低于target的投影后才能将该revision视为applied
- **AND** Pub/Sub丢失时周期性revision补偿 MUST使副本最终收敛
- **WHEN** watcher收到重复、相等或乱序通知
- **THEN** policy reload MUST保持幂等且不得倒退enforcer；消息kind要求的用户角色cache invalidation副作用 MUST仍按既有协议执行
- **AND** 定向user-role invalidation通知 MUST NOT独立推进Casbin engine applied revision或伪造policy reload完成

#### Scenario: applied revision、lag 与健康语义

- **WHEN** 系统报告本地applied revision、policy reload status或reload lag
- **THEN** local applied值 MUST来自engine当前实际授权投影，lag MUST计算为`max(known_latest_database_revision - engine_applied_revision, 0)`
- **AND** reload失败、消息接收或Redis revision更新 MUST NOT提升applied revision或将lag错误清零
- **WHEN** lag为0且latest revision已知
- **THEN** engine实际投影revision MUST不低于该latest revision，且最近reload状态 MUST成功，系统才可仅基于policy projection判定readiness/startup健康
- **WHEN** engine未初始化、最近reload失败或applied revision低于已知target
- **THEN** readiness/startup MUST报告policy不可用并拒绝业务流量

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

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或 route diff 生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应边界。RBAC watcher、cache、resolver 和 policy 投影资源 MUST 显式启动、停止和回滚；permission composition MUST 以单一 runtime 聚合对象表达稳定组件集合。

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

#### Scenario: watcher、cache 与 lifecycle

- **WHEN** user-service 启停 watcher 和 user-role resolver/cache
- **THEN** `NewWatcher` MUST 只构造对象，MUST NOT 启动 goroutine、订阅 Redis 或执行补偿循环
- **AND** resolver/cache 的 Fx result MUST 显式提供 `Start(context.Context) error` 与 `Close() error` lifecycle 视图，hook MUST NOT 通过 type assertion 探测启动能力
- **AND** hook MUST 先启动 resolver/cache，再初始加载 policy 并启动 watcher；resolver/cache 启动失败时 MUST 返回错误且 MUST NOT 继续
- **AND** `Start()` 和 `Stop(ctx)` MUST 幂等，`Stop(ctx)` MUST 取消内部 context 并在调用方期限内等待退出
- **AND** Stop 超时 MUST 返回 context 错误并保持重复停止安全
- **AND** 启动失败或停止时已启动 watcher MUST 被停止，cache MUST 幂等关闭
- **AND** watcher stop 和 cache close 同时失败时 hook MUST 保留全部 cause，且前者失败时仍 MUST 执行后者

#### Scenario: 共享资源所有权与关闭安全

- **WHEN** RBAC 关闭 watcher、cache、store 或 resolver
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源
- **AND** 关闭后授权 MUST 继续 fail-closed，不得因本地资源不可用产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置、权限基线或 key schema 下沉到 `common`

### Requirement: RBAC feature cache 配置与依赖边界

user-service MUST 私有拥有 user-role feature cache 的默认值、启用时校验和到通用 localcache 配置的集中映射。permission/RBAC 构造路径 MUST 只消费窄 RBAC settings，不得依赖完整 user-service 根配置；cache 禁用只能改变性能，授权必须继续 fail-closed。

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

### Requirement: RBAC policy sync 兼容 Redis Cluster

RBAC policy sync MUST 兼容 Redis Cluster。数据库 policy revision 写入 Redis cache 时使用的 key、policy refresh channel、周期性版本补偿和 Pub/Sub watcher MUST 使用稳定 hash tag 或 Cluster-compatible key schema，并只消费 Cluster-capable Redis client 或最小接口，MUST NOT 要求 `*redis.Client` 单机 concrete type。Redis MUST 只缓存和传播调用方提供的数据库 revision，MUST NOT 使用 counter 命令分配权威版本。

#### Scenario: 数据库 revision 发布与补偿

- **WHEN** 在线 RBAC 写操作提交新的数据库 policy revision
- **THEN** Redis version key MUST 位于稳定 hash tag 下，并允许 Redis Cluster client 写入该 revision
- **AND** adapter MUST NOT 使用 `INCR`、时间戳或本地计数器生成 revision，较小 revision MUST NOT 覆盖 Redis 中已存在的较大 revision
- **AND** 本地 reload、revision 发布和周期性 version check 的错误语义 MUST 保持 fail-closed、可恢复与可诊断

#### Scenario: Pub/Sub 通知与 watcher 生命周期

- **WHEN** watcher 订阅 policy refresh channel 或接收远端更新
- **THEN** channel 名称 MUST 使用稳定 hash tag 或 Cluster-compatible 命名
- **AND** watcher 停止、cache 关闭或 RBAC runtime 关闭 MUST NOT 关闭共享 Redis Cluster client

### Requirement: RBAC policy revision 提交顺序水位

系统 MUST 使在线 RBAC policy revision 的数值顺序与事务提交可见顺序一致；任一已提交 revision N MUST 表示所有已分配且小于 N 的在线 mutation 已经提交或回滚，数据库最大已提交 revision 才能作为完整授权快照水位。revision 分配、业务 mutation 和 outbox event MUST 保持同一 PostgreSQL transaction，Redis 或进程内状态 MUST NOT 参与权威 revision 分配。

#### Scenario: 并发 mutation 不得按 revision 逆序提交

- **WHEN** 两个在线 RBAC mutation 并发执行且较早 mutation 已获得 revision N 但尚未提交
- **THEN** 后续 mutation MUST NOT 以 revision N+1 先行提交
- **AND** 后续 mutation MUST 等待前一事务提交或回滚后再获得可提交 revision

#### Scenario: 升级后 revision 从已有最大值继续

- **WHEN** 数据库已经存在 policy revision 并应用 revision counter migration
- **THEN** counter MUST 以当前最大已提交 revision 初始化
- **AND** 新在线 mutation 分配的 revision MUST 大于全部已有 revision

### Requirement: 全局 policy 通知刷新当前权威快照

系统 MUST 对每条有效 `policy_changed` 通知执行当前 PostgreSQL 权威 policy 快照刷新。刷新 MAY 与同时发生的刷新合并，但 MUST NOT 仅因通知 revision 小于或等于 applied revision 而跳过；候选快照低于当前 target 时 MUST NOT 交换，相同 revision 的强制刷新候选 MUST 能更新其绑定的 enforcer。

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

### Requirement: revision gap 恢复全部用户角色缓存

系统 MUST 在 watcher 从较低 applied revision 直接追赶到较高数据库 revision 时，全量提升本实例 user-role cache generation；当前消息中的精确 user ID 不足以证明中间 revision 未包含其他用户绑定变更。仅当数据库 target 不高于 applied revision 时，重复 `user_role_changed` event MAY 只失效消息指定用户。

#### Scenario: 漏收前序用户绑定通知后收到更高 revision

- **WHEN** 实例漏收用户 A 的 `user_role_changed` event，随后收到用户 B 对应的更高数据库 revision
- **THEN** watcher MUST 追赶到数据库 revision 并失效全部 user-role cache
- **AND** 用户 A 的旧缓存 MUST NOT 因后续消息只包含用户 B 而永久保留

### Requirement: RBAC 写 API 准确表达数据库提交结果

在线 RBAC 写 API MUST 仅以业务 mutation transaction 是否提交决定 mutation 成败。transaction 提交后发生的本地 reload 或缓存失效错误 MUST 保持实例 fail-closed、记录已提交 revision 并由 outbox 自动恢复，但 MUST NOT 把已提交 mutation 向调用方表达为失败；成功响应所需数据 MUST 在提交 transaction 内产生或无需提交后数据库读取。

#### Scenario: 提交后本地 reload 失败

- **WHEN** RBAC mutation、revision 和 outbox 已提交，但本地 policy reload 失败
- **THEN** API MUST 返回该 mutation 的成功结果
- **AND** 本实例授权 MUST fail-closed，pending outbox MUST 保持可投递并在后台恢复 projection

#### Scenario: 提交前任一步失败

- **WHEN** 业务 mutation、revision counter、revision、outbox 或 transaction commit 任一步失败
- **THEN** API MUST 返回失败并且 transaction 内全部变化 MUST 回滚
- **AND** command MUST NOT执行提交后本地同步

#### Scenario: 绑定写响应不执行提交后查询

- **WHEN** 用户角色或角色权限 Add、Remove 或 Replace transaction 成功
- **THEN** store MUST 返回同一 transaction 内构造的最终绑定集合与 committed revision
- **AND** command MUST NOT 为构造成功响应在 commit 后重新查询数据库

### Requirement: RBAC 并发与故障验收覆盖真实链路

系统 MUST 使用真实 PostgreSQL transaction 验证 revision 提交顺序和 100 并发 mutation，并使用真实 outbox store、dispatcher、Redis publisher、watcher 与 Casbin engine 验证 Redis 故障恢复、重放和多副本最终授权收敛；仅并发调用 fake loader 或手工推进 fake revision 的测试 MUST NOT 作为这些验收场景的唯一证据。

#### Scenario: 一百个并发写最终收敛

- **WHEN** 100 个在线 RBAC mutation 并发提交，并对投递链路执行独立的可控 Redis publish 故障验收
- **THEN** 每个成功 mutation MUST 具有唯一 commit-ordered revision 和 pending outbox event
- **AND** Redis 恢复后无需新增 mutation，所有测试副本 MUST 最终应用数据库最大 revision 且授权结果对应最终关系数据

#### Scenario: Add Remove Replace 重放

- **WHEN** Add、Remove 和 Replace event 因 publish 后 ack 前故障被重复投递
- **THEN** dispatcher MUST 最终 ack 每个 event，watcher 副作用 MUST 保持幂等且不得丢失必要刷新或缓存失效

### Requirement: RBAC 业务接口限流门禁

系统 MUST 对受 RBAC 保护的权限、角色和用户业务接口执行认证后 User ID 限流。该限流 MUST 位于认证 middleware 之后、RBAC 授权 middleware 之前，并保持授权 fail-closed 语义不变。

#### Scenario: 限流发生在授权前

- **WHEN** 已认证请求访问权限、角色或用户业务接口且对应 User ID 已超出限流阈值
- **THEN** 系统 MUST 在调用 RBAC authorizer 前拒绝请求
- **AND** 响应 MUST 为 `429 Too Many Requests`、限流错误 code 和 `success=false`

#### Scenario: 未超限请求继续授权

- **WHEN** 已认证请求未超过 User ID 限流阈值并访问受 RBAC 保护接口
- **THEN** 系统 MUST 继续使用当前用户 ID、Gin route template 和 HTTP method 执行 RBAC 授权
- **AND** 授权失败、policy 不可用或用户角色回源失败 MUST 继续返回现有 fail-closed 授权错误，不得被限流逻辑吞掉

### Requirement: RBAC policy revision 与事务 outbox

系统 MUST 以 PostgreSQL 中追加式 `rbac_policy_revisions` 记录作为在线 RBAC policy revision 的唯一权威来源，并为每次成功的在线角色、角色权限或用户角色 mutation 持久化一条全局单调 revision 和一条唯一 pending outbox event。业务 mutation、revision 分配和 outbox 写入 MUST 在同一 PostgreSQL transaction 中提交；Redis MUST NOT 分配、替代或恢复权威 revision。

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
- **AND** API MAY 返回同步错误，但 mutation 的可靠恢复 MUST NOT 依赖该次 Redis 操作成功

#### Scenario: 离线写入边界

- **WHEN** RBAC seed、bootstrap 或受控 migration 修改离线系统数据
- **THEN** 本 change MUST NOT 要求这些离线流程伪装成在线 outbox dispatcher 或宣称已完成副本同步
- **AND** 运维 MUST 继续通过显式 reload 或滚动重启使授权投影收敛

### Requirement: RBAC policy outbox 可靠投递

系统 MUST 以 PostgreSQL 中已提交的 RBAC policy outbox event 作为跨副本 revision 通知的可靠恢复事实，并由显式 dispatcher 对到期 event 执行 claim、Redis publish、成功 ack 和失败退避。dispatcher MUST 提供至少一次投递并在进程崩溃、Redis 暂时不可用或 publish 失败后自动恢复；Redis MUST 只作为数据库 revision 通知的可重放加速层，不得成为 event、revision 或投递状态的权威来源。

#### Scenario: 到期事件被 claim 并成功投递

- **WHEN** pending 或 failed event 的 `next_attempt_at` 已到期且 dispatcher 正在运行
- **THEN** dispatcher MUST 按 revision 升序批量 claim event、发布同一数据库 revision 的 Redis 通知并条件标记 delivered
- **AND** delivered event MUST 记录完成时间、清除 claim 与最近错误，后续扫描 MUST NOT 再将其作为可投递事件返回

#### Scenario: Redis 故障后自动恢复

- **WHEN** Redis 不可用、version cache 更新失败或 Pub/Sub publish 失败
- **THEN** dispatcher MUST NOT 删除、吞掉或标记该 event 为 delivered
- **AND** 系统 MUST 记录失败 attempt、稳定错误摘要和下一次尝试时间，并按配置退避继续重试
- **WHEN** Redis 恢复且 event 再次到期
- **THEN** dispatcher MUST 无需新的 RBAC mutation 或人工复制 event 即可重新发布并最终 ack

#### Scenario: 进程重启与过期 lease 恢复

- **WHEN** dispatcher 在 claim 后、publish 中或 publish 成功但 ack 前停止或崩溃
- **THEN** event MUST 保留 processing 状态和持久 lease，且不得因进程内状态丢失而消失
- **WHEN** claim lease 到期
- **THEN** 任一健康 dispatcher MUST 能重新 claim 并继续处理该 event
- **AND** publish 成功但 ack 前崩溃 MAY 产生重复通知，consumer 副作用 MUST 保持幂等

#### Scenario: 多 dispatcher 并发 claim

- **WHEN** 多个 user-service 副本并发扫描同一批 due event
- **THEN** PostgreSQL claim MUST 通过行级仲裁为每个 event 建立唯一有效 claim token 与 lease
- **AND** 同一 lease 期间最多一个 owner MUST 获得该 event，其他 dispatcher MUST 跳过已 claim 行而非执行非幂等副作用
- **AND** ack 或失败更新 MUST 同时匹配 event、processing 状态和 claim token，过期 owner MUST NOT 覆盖新 owner 的处理结果

#### Scenario: 失败退避与保留

- **WHEN** 第 N 次实际 publish 尝试失败
- **THEN** attempt count MUST 增加一次，下一次尝试 MUST 使用不超过配置最大值的有界指数退避
- **AND** failed event MUST 持续保留且没有因达到固定 attempt 次数而进入不可恢复终态
- **AND** 无效 event 数据 MUST 作为可诊断失败保留并退避，MUST NOT 被静默 ack 或删除

### Requirement: RBAC revision 通知消息与幂等消费

Redis policy refresh 消息 MUST 使用显式版本化 envelope 携带稳定 event identity、数据库 `policy_revision`、change kind、reason 及相关对象 ID。publisher 和 watcher MUST 接受消息的重复与乱序，Redis revision cache 与本地 revision tracker MUST 只按 max 推进；系统 MUST NOT 保留旧 `INCR` counter 或旧消息 payload fallback。

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

### Requirement: RBAC policy sync 故障注入验收

系统 SHALL 提供可在 CI 中运行的 RBAC policy sync 故障注入验收测试，覆盖数据库 revision、同步通知、dispatcher、watcher、Casbin projection 和用户角色 cache 在故障、乱序、重放和并发写入下的最终收敛。测试 MUST 使用通道、barrier、eventually-style 条件或明确 deadline 控制并发时序，MUST NOT 使用固定 `time.Sleep` 作为状态已变化的主要判断依据。

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
- **AND** 文档 MUST 明确真实 PostgreSQL/Redis 容器依赖遵循 `AEGISCORE_TEST_CONTAINERS=1`

### Requirement: 用户角色缓存失效顺序门禁

系统 MUST 将 user-role cache 的回源写入与 per-user generation、全量 generation 或等价 revision 绑定。用户角色缓存失效 MUST 先提升对应 generation/revision，再删除或清空缓存项；任何在失效前开始、失效后完成的旧回源结果 MUST NOT 写入缓存。该门禁 MUST 位于 permission feature 的 resolver 或 cache wrapper 边界内，MUST NOT 将 RBAC 业务 revision 语义下沉到 `common/runtime/localcache` 公共 API。cache disabled 模式 MUST 继续直接回源并保持 fail-closed。

#### Scenario: 单用户失效抑制旧 load 写回

- **WHEN** 用户角色 cache miss 已经开始为某个用户回源，且该用户的 Add、Remove 或 Replace 用户角色绑定成功后调用 `InvalidateUserRole`
- **THEN** 系统 MUST 先提升该用户的 generation/revision，再删除该用户缓存项
- **AND** 失效前开始但失效后完成的旧回源结果 MUST NOT 写入该用户缓存
- **AND** 后续授权 MUST 重新回源并使用失效后的最终角色集合

#### Scenario: 全量失效抑制所有旧 load 写回
- **WHEN** 一个或多个用户角色 cache miss 已经开始回源，且系统调用 `InvalidateAllUserRoles`
- **THEN** 系统 MUST 先提升全量 generation/revision，再清空用户角色缓存
- **AND** 全量失效前开始但失效后完成的任一旧回源结果 MUST NOT 写入缓存
- **AND** 后续授权 MUST 重新回源并使用全量失效后的最终角色集合

#### Scenario: 失效竞态保持 fail-closed
- **WHEN** `RolesForUser` 的回源结果因 generation/revision 过期而被抑制
- **THEN** 当前授权请求 MUST fail-closed，MUST NOT 使用旧角色集合产生允许结果
- **AND** loader 错误、context 取消或过期回源结果 MUST NOT 写入缓存

#### Scenario: cache disabled 模式保持直接回源
- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 不创建通用 loading cache，并逐次从 PostgreSQL 回源当前启用角色
- **AND** `InvalidateUserRole` 与 `InvalidateAllUserRoles` MUST 保持安全且不得引入旧 load 写回路径
- **AND** 回源成功 MUST 返回独立角色 ID slice，回源错误或 context 取消 MUST 保持 fail-closed

### Requirement: RBAC watcher 以数据库 revision 补偿收敛

RBAC watcher MUST 以 PostgreSQL latest policy revision作为副本补偿和reload目标的权威来源，并以本地Casbin engine实际应用的projection revision作为本地状态。Redis Pub/Sub及其payload revision MUST只作为可丢失、可重复、可乱序的唤醒hint，Redis counter、key缺失或重建状态 MUST NOT决定副本已经收敛。授权热路径 MUST NOT因本要求增加PostgreSQL或Redis revision读取。

#### Scenario: Pub/Sub消息触发数据库revision校准

- **WHEN** watcher收到合法policy refresh消息
- **THEN** watcher MUST读取数据库latest policy revision并以该revision作为`ReloadToRevision`或等价revision-aware reload的目标
- **AND** payload revision MUST只作为hint和低风险诊断字段，MUST NOT直接推进local applied projection revision、清零lag或覆盖数据库latest revision
- **AND** payload重复、乱序或低于local applied revision时，engine投影 MUST保持不倒退，消息要求的既有cache side effect仍 MUST保持幂等语义

#### Scenario: Pub/Sub丢失后的周期补偿

- **WHEN** 数据库latest policy revision高于local applied projection revision且对应Pub/Sub消息丢失
- **THEN** 周期性`CheckVersion`或等价补偿检查 MUST直接读取数据库latest revision并触发revision-aware reload
- **AND** watcher MUST在后续成功检查与reload中最终使local applied projection revision不低于数据库latest revision
- **AND** 补偿判断 MUST NOT依赖Redis counter存在、领先或与数据库latest相等

#### Scenario: Redis状态不影响数据库补偿

- **WHEN** Redis counter不存在、落后于数据库latest、被重建为较小值或Redis从故障中恢复
- **THEN** watcher MUST继续以数据库latest revision判断是否需要reload
- **AND** 系统 MUST NOT因Redis值等于或低于local applied revision而跳过数据库revision超前所需的补偿
- **AND** Redis恢复后收到的旧消息 MUST NOT使旧revision覆盖新projection或降低local applied revision

#### Scenario: 数据库revision source不可用

- **WHEN** Pub/Sub唤醒或周期检查无法读取数据库latest policy revision
- **THEN** watcher MUST记录稳定的revision store unavailable诊断并保留底层cause用于日志
- **AND** watcher MUST NOT使用Redis counter或payload revision冒充数据库目标、记录reload success或把lag重置为`0`
- **AND** 后续数据库读取恢复时，周期检查或下一条hint MUST重新校准latest revision并继续补偿

#### Scenario: reload失败后恢复

- **WHEN** 数据库latest revision高于local applied revision但本地reload失败、被取消或未达到目标
- **THEN** engine MUST保留上一成功projection及其applied revision并保持fail-closed健康语义，watcher MUST记录reload failure且不得宣称收敛
- **AND** 后续Pub/Sub hint或周期检查 MUST再次读取数据库latest revision并重试
- **WHEN** 后续reload成功且实际applied revision不低于读取到的database latest revision
- **THEN** watcher MUST记录reload success并恢复收敛状态

#### Scenario: revision查询依赖边界

- **WHEN** permission feature查询latest policy revision
- **THEN** application MUST拥有只读最小revision source port，PostgreSQL/Ent adapter MUST留在permission infrastructure，named database与lifecycle选择 MUST留在composition
- **AND** revision查询语义 MUST NOT下沉到`common/`、`internal/shared/`或`internal/integration/`，application/domain MUST NOT导入Ent concrete client或predicate包
- **AND** 系统 MUST复用现有policy revision schema，MUST NOT为watcher新增revision、outbox schema或dispatcher

