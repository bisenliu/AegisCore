## Purpose

定义 user-service 的 RBAC 访问控制能力，覆盖权限目录、角色、角色权限、用户角色、Casbin 授权、策略同步、系统数据引导和超级管理员管理。

## Requirements

### Requirement: 权限目录与路由诊断

系统 MUST 提供权限创建、更新、启停、详情、列表和路由差异诊断能力。权限 MUST 使用稳定业务标识描述可授权的 HTTP method、route template 和业务模块；公开写接口 MUST NOT 允许调用方创建或篡改系统权限。

#### Scenario: 创建普通权限

- **WHEN** 授权调用方提交合法且唯一的权限标识、HTTP method、route template、模块和描述
- **THEN** 系统 MUST 创建可供角色绑定和策略加载使用的非系统权限
- **AND** 成功响应 MUST 返回新建权限实体

#### Scenario: 公开接口不能创建系统权限

- **WHEN** 调用方通过公开权限创建接口提交系统权限标记
- **THEN** 系统 MUST 拒绝未知字段或忽略该字段并创建非系统权限
- **AND** 系统 MUST NOT 因调用方输入创建系统权限

#### Scenario: 更新和启停权限

- **WHEN** 授权调用方使用合法输入更新、启用或停用存在的普通权限
- **THEN** 系统 MUST 持久化对应变更
- **AND** 成功响应 MUST NOT 包含更新后的权限实体，调用方 MUST 通过查询接口读取最新状态

#### Scenario: 权限输入或目标无效

- **WHEN** 权限标识、HTTP method、route template、模块或描述不满足 domain validation，或者目标权限不存在或受系统保护
- **THEN** 系统 MUST 拒绝变更并返回可区分的业务错误

#### Scenario: 查询权限目录

- **WHEN** 授权调用方查询权限详情或按模块、HTTP method、状态、系统标记分页查询权限
- **THEN** 系统 MUST 按稳定权限 ID 排序返回当前权限数据和共享 pagination 信息

#### Scenario: 诊断路由差异

- **WHEN** 系统比较已注册的可授权 HTTP 路由与权限目录
- **THEN** 系统 MUST 返回 missing、stale 和不一致的权限定义
- **AND** 诊断 MUST NOT 创建权限、修改权限状态或改变角色绑定

#### Scenario: 发现可授权路由

- **WHEN** 系统构造路由差异诊断输入
- **THEN** 系统 MUST 排除 `OPTIONS`、`/api/v1/` 之外的路由以及认证公开或会话控制路由
- **AND** application 层 MUST NOT 直接依赖 Gin engine

### Requirement: 角色目录与角色权限绑定

系统 MUST 提供角色创建、更新、启停、详情、列表和角色权限绑定能力。公开写接口 MUST NOT 允许调用方创建或篡改系统角色；角色权限关系 MUST 只引用存在且启用的权限，并在完整替换时保持事务性。

#### Scenario: 创建普通角色

- **WHEN** 授权调用方提交合法角色信息和可用权限集合
- **THEN** 系统 MUST 创建非系统角色并写入角色权限绑定
- **AND** 成功响应 MUST 返回新建角色实体

#### Scenario: 公开接口不能创建系统角色

- **WHEN** 调用方通过公开角色创建接口提交系统角色标记
- **THEN** 系统 MUST 拒绝未知字段或忽略该字段并创建非系统角色
- **AND** 系统 MUST NOT 因调用方输入创建系统角色

#### Scenario: 更新和启停角色

- **WHEN** 授权调用方使用合法输入更新、启用或停用存在的普通角色
- **THEN** 系统 MUST 持久化对应变更
- **AND** 成功响应 MUST NOT 包含更新后的角色实体，调用方 MUST 通过查询接口读取最新状态

#### Scenario: 系统角色受保护

- **WHEN** 普通角色接口尝试修改或停用系统角色
- **THEN** 系统 MUST 拒绝操作并保持系统角色及其基线语义不变

#### Scenario: 查询角色

- **WHEN** 授权调用方查询角色详情或分页查询角色
- **THEN** 系统 MUST 返回角色数据、权限摘要和共享 pagination 信息

#### Scenario: 绑定权限前校验

- **WHEN** 角色权限写请求引用不存在或已停用的权限
- **THEN** 系统 MUST 拒绝写入并保持已有角色权限关系不变
- **AND** role application MUST 通过 permission application 拥有的最小查询端口校验权限，MUST NOT 导入 permission infrastructure

#### Scenario: 完整替换角色权限

- **WHEN** 授权调用方使用合法权限集合替换角色的完整权限绑定
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一写入发生非幂等错误时系统 MUST 回滚全部变更

#### Scenario: 停用角色不再授权

- **WHEN** 角色被停用
- **THEN** 该角色 MUST NOT 出现在用户有效角色、有效权限或 Casbin policy 中

#### Scenario: 维护系统角色权限基线

- **WHEN** seed 补齐或同步系统角色权限绑定
- **THEN** 系统 MUST 批量维护绑定并把已有绑定视为幂等成功
- **AND** 非唯一冲突错误 MUST 使本次事务失败

### Requirement: 用户角色绑定与有效权限

系统 MUST 支持查询、添加、移除和完整替换用户角色绑定，并基于用户当前启用角色及其启用权限提供有效权限。写路径 MUST 校验用户和角色状态，失败时 MUST 保持原绑定和同步状态不变。

#### Scenario: 绑定启用角色

- **WHEN** 授权调用方将存在且启用的角色绑定给存在且未软删除的用户
- **THEN** 系统 MUST 写入用户角色关系并使后续授权能够使用该角色

#### Scenario: 绑定目标无效

- **WHEN** 用户角色写请求引用不存在或已软删除的用户、不存在的角色或已停用角色
- **THEN** 系统 MUST 拒绝写入并返回明确错误
- **AND** 系统 MUST NOT 改变已有关系、失效缓存或发送 policy change 通知

#### Scenario: 完整替换用户角色

- **WHEN** 授权调用方使用全部合法且启用的角色集合替换用户的完整角色绑定
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一角色不可用或任一写入失败时系统 MUST 回滚全部变更

#### Scenario: 查询用户有效权限

- **WHEN** 系统或授权调用方查询用户有效权限
- **THEN** 系统 MUST 聚合该用户当前启用角色和这些角色的启用权限并返回去重后的权限集合

#### Scenario: 用户没有有效角色

- **WHEN** 已认证用户没有有效角色绑定并访问受 RBAC 保护的路由
- **THEN** 系统 MUST 拒绝访问

### Requirement: RBAC 持久化完整性

系统 MUST 使用外部 UUID 作为角色、权限、用户角色和角色权限的稳定业务标识，并通过数据库约束、事务和与查询路径匹配的索引维护关系完整性和可预测性能。join 表内部主键或外键 MUST NOT 成为 feature 对外业务标识。

#### Scenario: 使用外部 UUID 查询和关联

- **WHEN** 系统创建、查询、排序或关联角色、权限及其绑定
- **THEN** 系统 MUST 使用当前外部 UUID 字段作为业务标识
- **AND** 系统 MUST NOT 向 application 或 transport 暴露 join 表内部标识

#### Scenario: 约束错误映射

- **WHEN** 持久化操作发生唯一冲突、引用目标不存在或目标状态不可用
- **THEN** adapter MUST 将数据库结果映射为对应领域错误
- **AND** application 层 MUST NOT 依赖 Ent predicate 或数据库错误细节

#### Scenario: 绑定替换失败

- **WHEN** 用户角色或角色权限完整替换在校验、删除或新增阶段失败
- **THEN** 系统 MUST 回滚事务并保留替换前的完整绑定

#### Scenario: 查询路径具备索引

- **WHEN** 系统执行权限或角色分页过滤、用户启用角色回源、角色权限加载或关系反向查询
- **THEN** Ent schema 和 Atlas migration MUST 提供与过滤字段、关系字段和稳定 ID 排序匹配的索引
- **AND** 索引调整 MUST NOT 改变授权结果

### Requirement: Casbin 授权与 HTTP 边界

系统 MUST 在认证通过后使用 RBAC 授权中间件保护权限、角色和用户业务接口。授权 MUST 使用用户与角色的稳定 subject、Gin route template object 和 HTTP method action，并在任何身份、策略或执行异常下 fail-closed。

#### Scenario: 构造授权请求

- **WHEN** 已认证请求进入受 RBAC 保护的 `/api/v1` 路由
- **THEN** 中间件 MUST 使用请求上下文中的用户 ID、Gin `FullPath()` 和 HTTP method 构造授权请求
- **AND** 用户 subject MUST 使用 `user:<user_uuid>`，角色 subject MUST 使用 `role:<role_uuid>`

#### Scenario: 允许和拒绝访问

- **WHEN** 用户当前启用角色拥有匹配 HTTP method 和 route template 的启用权限
- **THEN** 系统 MUST 允许请求进入目标 controller
- **AND** 没有匹配权限时系统 MUST 返回禁止访问错误

#### Scenario: 认证 subject 无效

- **WHEN** 请求缺少用户 ID、用户 ID 类型非法或 subject 不能解析为用户 UUID
- **THEN** 系统 MUST 返回未认证错误并拒绝请求
- **AND** 系统 MUST NOT 调用底层 Casbin engine

#### Scenario: 授权执行异常

- **WHEN** policy 未加载、用户角色回源失败、context 取消或 Casbin 执行返回错误
- **THEN** 系统 MUST 拒绝请求并返回内部错误
- **AND** 系统 MUST NOT 将执行异常折叠为允许结果

#### Scenario: 公开和预检请求旁路

- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS` 方法
- **THEN** 中间件 MUST 允许请求继续处理并 MUST NOT 调用授权服务

#### Scenario: policy 权威来源

- **WHEN** policy loader 构造授权策略
- **THEN** policy MUST 从启用角色、启用权限、角色权限绑定和用户角色绑定派生
- **AND** 独立 `casbin_rules` 表 MUST NOT 成为业务权威来源，用户身份解析 MUST 排除已软删除用户

#### Scenario: 超级管理员通配授权

- **WHEN** 用户拥有 `internal/shared/rbacbaseline` 定义的内置超级管理员角色
- **THEN** policy loader MUST 为该角色提供受保护业务接口的 wildcard policy
- **AND** 超级管理员角色常量 MUST 只由 `rbacbaseline` 提供

#### Scenario: 路由注册安全依赖完整

- **WHEN** user-service 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 授权中间件链
- **AND** token version validator、RBAC authorizer 或必需 controller 缺失时系统 MUST 拒绝注册部分业务路由

### Requirement: 授权热路径用户角色缓存

系统 MUST 使用有容量上限的本地 loading cache 缓存用户当前启用角色 ID 集合，并通过主动失效和全量清空使在线 RBAC 变更及时收敛。关闭缓存 MUST 只影响性能，不得改变授权结果；启用和 disabled 模式 MUST 提供相同的显式幂等关闭契约。

#### Scenario: 缓存命中

- **WHEN** 用户角色缓存命中
- **THEN** 授权 MUST 使用缓存中的角色 ID 副本
- **AND** 调用方对返回 slice 的修改 MUST NOT 污染缓存内部值

#### Scenario: 缓存未命中

- **WHEN** 用户角色缓存未命中
- **THEN** 系统 MUST 合并同一用户的并发回源并查询 PostgreSQL 中的当前启用角色
- **AND** loader 错误 MUST NOT 写入缓存

#### Scenario: 缓存配置和容量

- **WHEN** user-service 装配启用的用户角色缓存
- **THEN** 系统 MUST 从 `rbac.user_role_cache` 读取正值 `size`、`ttl` 和 `load_timeout`
- **AND** 未显式配置时 MUST 使用 `enabled=true`、`size=100000`、`ttl=5s` 和 `load_timeout=500ms`
- **AND** 容量淘汰、准入拒绝或 TTL 过期后 MUST 能通过 PostgreSQL 回源

#### Scenario: 关闭缓存

- **WHEN** `rbac.user_role_cache.enabled` 为 false
- **THEN** `size`、`ttl` 和 `load_timeout` MAY 为零值
- **AND** 系统 MUST 直接回源并保持正确的 fail-closed 授权语义
- **AND** disabled 模式 MUST 提供与启用模式相同的幂等 `Close` 契约

#### Scenario: 显式关闭缓存

- **WHEN** 调用方对启用或 disabled user-role resolver/cache 调用 `Close`
- **THEN** `Close` MUST 是幂等的，并且 MUST NOT 关闭调用方注入的 Redis、Ent 或 PostgreSQL 共享资源
- **AND** 关闭后授权语义 MUST 继续 fail-closed，不得因为关闭本地缓存而产生允许结果

#### Scenario: 在线绑定变更失效

- **WHEN** 在线用户角色添加、替换或移除成功
- **THEN** 本实例 MUST 失效对应用户缓存
- **AND** 其他副本 MUST 通过 policy sync 感知变更并失效缓存

#### Scenario: policy reload 清空缓存

- **WHEN** RBAC policy reload 或全量策略刷新完成
- **THEN** 系统 MUST 清空本实例用户角色缓存
- **AND** 后续请求 MUST 通过回源重新建立本地投影

### Requirement: Policy 加载与多副本同步

系统 MUST 在启动时通过显式初始化入口加载初始 policy，并在在线 RBAC 写操作成功后刷新本实例状态，再通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本。授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

#### Scenario: 启动加载 policy

- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 显式调用 initial load 初始化入口，并将可取消或带超时的启动 context 传给 policy loader
- **AND** permission infrastructure MUST NOT 通过 `RegisterInitialLoad(fx.Lifecycle, ...)` 或等价 Fx lifecycle adapter 注册初始加载
- **AND** loader MUST 能观察启动超时或取消

#### Scenario: 初始加载失败

- **WHEN** 初始 policy 加载失败或被取消
- **THEN** engine MUST 记录最近错误和 reload 失败指标
- **AND** 后续授权 MUST fail-closed，服务 MUST 保持既有启动语义
- **AND** reload 状态和 readiness 可观测性 MUST 保留失败信息

#### Scenario: 在线权限或角色变更

- **WHEN** 权限、角色状态或角色权限绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST reload policy 并发布新的 Redis policy version 和 Pub/Sub 通知

#### Scenario: 在线用户角色变更

- **WHEN** 用户角色绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST 失效相关用户缓存并通知其他副本刷新授权投影

#### Scenario: 同步失败向调用方传播

- **WHEN** 持久化成功后的 reload、缓存失效、version 发布或通知失败
- **THEN** command service MUST 向调用方返回同步错误，成功响应 MUST NOT 掩盖该错误
- **AND** `PolicyChangeNotifier` MUST 是正式 command service 的必需依赖

#### Scenario: reload 和发布局部失败

- **WHEN** policy refresh coordinator 执行本地 reload 和 version 发布
- **THEN** 本地 reload 失败后系统 MUST 仍尝试发布 version
- **AND** 两者同时失败时返回错误 MUST 保留两项失败
- **AND** 只有两者均成功时系统才 MUST 标记本实例已应用该 version

#### Scenario: watcher 生命周期

- **WHEN** user-service 启停 Redis policy watcher
- **THEN** `NewWatcher` MUST 只构造 watcher 对象，MUST NOT 启动 goroutine、订阅 Redis 或执行版本补偿循环
- **AND** `Start()` MUST 是幂等的，并使用内部可取消 context 驱动长期循环
- **AND** `Stop(ctx)` MUST 是幂等的，取消内部 context，并在调用方 context 限制内等待循环退出

#### Scenario: watcher 停止超时

- **WHEN** 调用方传入的 `Stop(ctx)` context 在 watcher 循环退出前到期
- **THEN** `Stop(ctx)` MUST 返回 context 相关错误
- **AND** watcher MUST 保持后续重复 `Stop(ctx)` 可安全调用，且 MUST NOT 关闭调用方注入的 Redis、Ent 或 PostgreSQL 共享资源

#### Scenario: 其他副本收敛

- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现远端 policy version 更新
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛

### Requirement: RBAC 系统数据与运维 CLI

系统 MUST 提供带服务上下文的 `rbac seed`、`rbac assign-super-admin` 和 `rbac create-super-admin` 命令，用于维护系统角色、系统权限、默认绑定和超级管理员。系统数据 MUST 只由 seed port 根据 `internal/shared/rbacbaseline` 写入。

#### Scenario: 初始化系统基线

- **WHEN** 运维执行 `aegiscore-user-services rbac seed`
- **THEN** 系统 MUST 幂等创建或更新基线角色、权限和绑定并输出变更统计
- **AND** 系统角色和权限 MUST 标记为系统数据
- **AND** seed MUST NOT 创建业务用户或自动分配超级管理员

#### Scenario: 普通路径不能写系统数据

- **WHEN** 非 seed 的角色或权限 command、store create 或公开 HTTP 路径写入数据
- **THEN** 系统 MUST 固定写入非系统数据并 MUST NOT 接收系统标记

#### Scenario: 分配超级管理员

- **WHEN** 运维执行 `rbac assign-super-admin --user-id <uuid>`
- **THEN** 系统 MUST 为指定存在用户幂等绑定内置超级管理员角色

#### Scenario: 创建超级管理员

- **WHEN** 运维执行 `rbac create-super-admin` 并提供合法 username 和密码
- **THEN** 系统 MUST 创建或复用用户并绑定内置超级管理员角色
- **AND** username MUST trim 后转为小写，空 nickname MUST 回退为归一化 username

#### Scenario: 超级管理员密码处理

- **WHEN** create-super-admin 未显式指定 password env
- **THEN** 系统 MUST 从 `ADMIN_PASSWORD` 读取非空密码
- **AND** 已有用户的密码 MUST NOT 默认重置，只有显式 `--reset-password` 或 `ADMIN_RESET_PASSWORD=true` 时系统才 MUST 更新密码
- **AND** 必需输入缺失时命令 MUST 返回明确错误

#### Scenario: 离线命令不等同在线刷新

- **WHEN** HTTP 副本运行期间执行 seed、assign-super-admin 或 create-super-admin
- **THEN** 命令 MUST 只修改持久化数据并 MUST NOT 宣称已触发运行期 policy refresh
- **AND** 运维 MUST 滚动重启副本或触发在线 RBAC 刷新使运行实例收敛

### Requirement: RBAC 应用错误与统一 HTTP 渲染

permission、role 和 binding domain MUST 返回携带稳定 HTTP status、共享业务 code、公开 message 和 `Reason` 的应用错误。HTTP transport MUST 通过共享 `response.Fail` 直接渲染业务错误，MUST NOT 维护 feature 专用 sentinel-to-HTTP mapper；直接或包装返回的应用错误 MUST 保留 `errors.Is` 语义。

#### Scenario: 权限目录错误

- **WHEN** permission feature 返回权限已存在、权限不存在、输入无效或系统权限保护错误
- **THEN** 系统 MUST 分别使用 `409 Conflict`、`404 Not Found`、`400 Bad Request`、`409 Conflict`
- **AND** `Reason` MUST 分别为 `permission_already_exists`、`permission_not_found`、`permission_invalid`、`system_permission_protected`

#### Scenario: 角色目录错误

- **WHEN** role feature 返回角色已存在、角色不存在、输入无效、系统角色保护或角色停用错误
- **THEN** 系统 MUST 分别使用 `409 Conflict`、`404 Not Found`、`400 Bad Request`、`409 Conflict`、`409 Conflict`
- **AND** `Reason` MUST 分别为 `role_already_exists`、`role_not_found`、`role_invalid`、`system_role_protected`、`role_inactive`

#### Scenario: 用户角色绑定错误

- **WHEN** 用户角色增量绑定或解绑返回绑定已存在或绑定不存在错误
- **THEN** 系统 MUST 分别返回 `409 Conflict` 或 `404 Not Found`
- **AND** `Reason` MUST 分别为 `user_role_already_exists` 或 `user_role_not_found`

#### Scenario: 角色权限绑定错误

- **WHEN** 角色权限增量绑定或解绑返回绑定已存在或绑定不存在错误
- **THEN** 系统 MUST 分别返回 `409 Conflict` 或 `404 Not Found`
- **AND** `Reason` MUST 分别为 `role_permission_already_exists` 或 `role_permission_not_found`

#### Scenario: 跨 feature 错误透传

- **WHEN** role 流程收到 `identity.ErrUserNotFound` 或 permission 的不存在错误
- **THEN** role HTTP transport MUST 通过共享 response helper 保留错误自身的 status、code 和 message
- **AND** role transport MUST NOT 复制 identity 或 permission 错误映射

#### Scenario: controller 统一错误出口

- **WHEN** permission 或 role controller 的 command/query 返回业务错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** transport MUST NOT 调用或保留 `toPermissionHTTPError`、`toRoleHTTPError` 或等价 mapper

### Requirement: RBAC 可观测性

系统 MUST 为 RBAC 授权判定和正式模块执行的 route diff 提供低基数 metrics，并使用显式注入的 logger 记录加载和同步异常。观测失败 MUST NOT 改变授权或策略同步结果。

#### Scenario: 记录授权结果和耗时

- **WHEN** permission authorization service 完成一次 RBAC Enforce 判定
- **THEN** counter MUST 记录 `result="allow"`、`result="deny"` 或 `result="error"`
- **AND** histogram MUST 记录本次判定耗时
- **AND** 标签 MUST 只使用 `result`、HTTP method 和 route template

#### Scenario: 指标禁止高基数数据

- **WHEN** 系统记录 RBAC metrics
- **THEN** 指标 MUST NOT 包含用户、角色、权限、token、trace、IP、账号、Redis key、SQL、原始错误或 raw path
- **AND** route 标签 MUST 使用稳定 route template

#### Scenario: 记录 route diff

- **WHEN** 正式 permission 模块执行 route diff
- **THEN** 系统 MUST 记录本次 missing、stale 和不一致结果
- **AND** 指标记录 MUST NOT 修改权限目录或路由诊断结果

#### Scenario: 显式日志依赖

- **WHEN** role 或 permission application、policy loader、watcher、cache 或 adapter 需要记录日志
- **THEN** logger MUST 由 constructor 显式注入或由调用方 context 提供
- **AND** 生产主路径 MUST NOT 依赖 package-level 默认 logger
- **AND** 日志 MUST 使用稳定低基数字段并 MUST NOT 记录 token、SQL、Redis key 或原始 policy 数据

### Requirement: RBAC 分层与组合边界

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL 和 HTTP response 细节 MUST 留在对应 composition、transport 或 infrastructure 边界。role infrastructure store constructor MUST 使用显式普通 Go 参数接收 Ent client 和必要的消费侧窄 port，MUST NOT 通过 `fx.In`、`dig.In`、`fx.Out`、`dig.Out`、`name` tag 或其他 DI metadata 表达依赖。permission infrastructure 可以拥有 Ent、Redis、Casbin 和 cache 的具体适配细节，但 MUST NOT 依赖 Fx 或 Dig。

#### Scenario: application 直接构造

- **WHEN** role 或 permission application service 在单元测试或非 Fx 调用方中构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明仅服务于 DI 的 tag

#### Scenario: feature composition 组装依赖

- **WHEN** 正式 feature module 注册 application service、policy engine、watcher、cache 和 adapter
- **THEN** 无 DI metadata 的构造器 MUST 直接注册
- **AND** named、optional 或配置转换 adapter MUST 留在 feature composition 边界
- **AND** 必需安全依赖缺失时 graph MUST 构造失败，MUST NOT 静默降级

#### Scenario: role store adapter 显式构造

- **WHEN** 调用方构造 `RoleStore`、`RolePermissionStore` 或 `UserRoleStore`
- **THEN** 调用方 MUST 以普通 Go 参数显式传入 `*ent.Client`
- **AND** constructor MUST NOT 暴露或接收 `fx.In`、`dig.In`、`fx.Out`、`dig.Out` 或 `name:"primary_db"` 等 DI metadata
- **AND** `PermissionLookup` 等跨 feature 依赖 MUST 继续通过 role application 消费侧窄 port 显式注入，MUST NOT 扩大为 permission infrastructure 宽接口

#### Scenario: role feature 禁止 Fx/Dig 回归

- **WHEN** 执行 `user-service-architecture-lint`
- **THEN** lint MUST 检查 `user-service/internal/features/role` 的 domain、application、infrastructure 和 transport 生产 Go 文件
- **AND** 这些文件 MUST NOT import `go.uber.org/fx` 或 `go.uber.org/dig`
- **AND** 这些文件 MUST NOT 使用 `fx.In`、`fx.Out`、`dig.In`、`dig.Out` 或仅服务于 DI 的 tag
- **AND** role feature 的 `fx.go` 与 `fx_test.go` MAY 继续作为 composition 和 graph 验证边界使用 Fx

#### Scenario: 显式生命周期由 composition 编排

- **WHEN** 正式 feature module 需要启动 initial load、RBAC watcher 或关闭 user-role cache
- **THEN** Fx lifecycle hook MUST 只登记在 composition 层，并且 MUST 调用 infrastructure 对象暴露的显式 `Initialize`、`Start`、`Stop` 或 `Close` 方法
- **AND** `user-service/internal/features/permission/infrastructure` 的生产代码 MUST NOT import `go.uber.org/fx` 或 `go.uber.org/dig`，也 MUST NOT 使用 `fx.Lifecycle`、`fx.Hook`、`fx.In` 或 `fx.Out`

#### Scenario: 共享边界保持最小

- **WHEN** role 需要查询权限或 permission 需要解析用户身份
- **THEN** 消费侧 application MUST 定义最小 port 并由相邻 feature 或 integration adapter 实现
- **AND** feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport

#### Scenario: 服务资源归属

- **WHEN** user-service 装配 RBAC 的 PostgreSQL、Redis 和用户角色缓存
- **THEN** 具名资源 MUST 来自服务自有 `resources.postgres` 和 `resources.redis`，feature cache MUST 来自 `rbac.user_role_cache`
- **AND** RBAC MUST NOT 把服务业务配置或 key schema 下沉到 `common`
- **AND** watcher 和 cache 的 `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源

#### Scenario: 架构调整不改变行为

- **WHEN** provider 注册、依赖投影、logger 注入、application 构造方式或显式生命周期编排调整
- **THEN** 权限、角色、绑定、授权、policy reload、缓存失效、跨副本同步和 CLI 行为 MUST 保持不变
- **AND** 架构检查 MUST 阻止 application/domain 引入框架依赖或生产代码重新依赖全局 logger

### Requirement: Permission adapter 显式装配边界

permission 的 PostgreSQL、Redis 和 Casbin infrastructure adapter 构造 API MUST 使用普通 Go 参数表达必需依赖，并 MUST NOT 在 adapter constructor 中暴露或要求 Fx/Dig metadata。生产 Fx composition MAY 在 feature composition 边界选择具名资源和生命周期挂钩，但 MUST 通过显式 Go 赋值暴露 concrete 与 application/authorization port 视图。

#### Scenario: adapter constructor 不携带 DI metadata

- **WHEN** 构造 permission `PermissionStore`、policy `Loader`、Casbin `Engine` 或 Redis policy `Store`
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options
- **AND** constructor MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、`fx.As`、`fx.Self`、named result 或 group result

#### Scenario: composition 显式选择服务资源

- **WHEN** 正式 permission Fx module 装配 PostgreSQL、Redis、policy loader、policy store、version tracker 或 authorization engine
- **THEN** 具名 `primary_db`、`cache_redis` 或生命周期依赖的选择 MUST 留在 `features/permission/fx.go` composition 边界
- **AND** PostgreSQL、Redis 和 Casbin adapter package 的生产构造 API MUST NOT import Fx 或 Dig 只为读取这些 tags

#### Scenario: 同一 Engine 暴露多个端口

- **WHEN** composition 需要同时提供 Casbin concrete `Engine`、`permissionauthorization.Engine` 和 `permissionapplication.PolicyReloadEngine`
- **THEN** composition MUST 构造一个 `Engine` 实例并通过普通 Go 赋值暴露这些端口
- **AND** 系统 MUST NOT 为 concrete、authorization port 或 reload port 重复构造有状态 `Engine`

#### Scenario: 同一 Redis Store 暴露发布端口

- **WHEN** composition 需要同时提供 Redis policy `Store` concrete 视图和 `permissionapplication.PolicyVersionPublisher` 等接口视图
- **THEN** composition MUST 构造一个 `Store` 实例并通过普通 Go 赋值暴露这些端口
- **AND** 系统 MUST NOT 为 concrete 和 interface 视图重复构造有状态 Redis policy store 或 version tracker

#### Scenario: 行为保持不变

- **WHEN** permission adapter 构造 API 从 Fx/Dig metadata 改为普通 Go 参数
- **THEN** 权限目录、route diff、Casbin policy、授权 fail-closed、Redis policy version、Pub/Sub、用户角色缓存失效和多副本同步语义 MUST 保持不变
- **AND** 本变更 MUST NOT 迁移 Casbin initial load、watcher `Start/Stop` 或用户角色缓存 `Close` 生命周期
