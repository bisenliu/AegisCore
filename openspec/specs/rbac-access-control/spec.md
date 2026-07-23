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

系统 MUST 在认证后使用 RBAC 中间件保护权限、角色和用户业务接口，并以 PostgreSQL 关系数据作为业务权威投影，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。授权 MUST 使用稳定 subject、Gin route template 和 HTTP method，并在任何身份、策略或执行异常下 fail-closed。系统 MUST 在启动时显式加载 policy，在线写成功后刷新本实例，并通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本；授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

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
- **WHEN** 用户角色回源失败、context 取消、Casbin 执行错误、policy 未加载或最近一次 reload 失败
- **THEN** 系统 MUST 拒绝请求并暴露 policy 不可用 readiness/startup 状态，MUST NOT 将异常或 policy 缺失折叠为允许结果
- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS`
- **THEN** 中间件 MUST 允许请求并 MUST NOT 调用授权服务
- **WHEN** 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 中间件链；token version validator、RBAC authorizer 或必需 route registrar 缺失时系统 MUST 拒绝注册部分路由

#### Scenario: policy 权威来源、初始加载与恢复

- **WHEN** policy loader 构造授权策略
- **THEN** policy MUST 从启用角色、角色权限绑定、permissions 投影和用户角色绑定派生
- **AND** loader MUST NOT 使用权限 active predicate，独立 `casbin_rules` 表 MUST NOT 成为业务权威来源，用户身份解析 MUST 排除已软删除用户
- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 显式调用初始加载入口，并将可取消或带超时的启动 context 传给 loader
- **WHEN** 初始加载失败或被取消
- **THEN** engine MUST 记录最近错误和 reload 失败指标，后续授权 MUST fail-closed，`app.Start` MUST 保持成功
- **AND** reload 状态和 readiness/startup MUST 保留失败信息并拒绝接入业务流量
- **WHEN** 后续 Pub/Sub、版本补偿或显式 reload 成功
- **THEN** engine MUST 替换为最新 policy、清除最近错误并恢复 readiness/startup

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
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice 并保持 fail-closed；direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次结果

#### Scenario: 在线写后同步与离线变更

- **WHEN** 角色状态、角色权限或用户角色绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST reload policy 或失效相关用户缓存，并发布新的 Redis policy version 和 Pub/Sub 通知
- **AND** reload、缓存失效、version 发布或通知失败 MUST 向调用方返回同步错误，成功响应 MUST NOT 掩盖错误
- **AND** `PolicyChangeNotifier` MUST 是正式 command service 的必需依赖
- **WHEN** 权限投影由离线 migration、seed 或 bootstrap 改变
- **THEN** 离线命令 MUST NOT 宣称已完成在线 policy refresh，运维 MUST 显式 reload 或滚动重启副本

#### Scenario: reload 发布与副本收敛

- **WHEN** coordinator 执行本地 reload 和 version 发布
- **THEN** 本地 reload 失败后系统 MUST 仍尝试发布 version
- **AND** 两者同时失败时错误 MUST 保留两项失败，只有两者均成功时系统才 MUST 标记本实例已应用该 version
- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现远端 version 更新
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛

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
