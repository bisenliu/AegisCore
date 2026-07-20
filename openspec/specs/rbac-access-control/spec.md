## Purpose

定义 user-service 的 RBAC 访问控制能力，覆盖权限目录、角色、角色权限、用户角色、Casbin 授权、策略同步、系统数据引导、超级管理员管理、错误契约、观测和资源生命周期。

## Requirements

### Requirement: 权限目录与路由诊断

系统 MUST 提供权限创建、更新、启停、详情、列表和路由差异诊断能力。权限 MUST 使用稳定业务标识描述可授权的 HTTP method、route template 和业务模块；公开写接口 MUST NOT 允许调用方创建或篡改系统权限。

#### Scenario: 权限目录写入和查询

- **WHEN** 授权调用方提交合法且唯一的权限标识、HTTP method、route template、模块和描述
- **THEN** 系统 MUST 创建可供角色绑定和策略加载使用的非系统权限，并返回新建权限实体
- **WHEN** 授权调用方使用合法输入更新、启用或停用存在的普通权限
- **THEN** 系统 MUST 持久化对应变更，成功响应 MUST NOT 包含更新后的权限实体
- **WHEN** 授权调用方查询权限详情或分页过滤权限
- **THEN** 系统 MUST 按稳定权限 ID 排序返回当前权限数据和共享 pagination 信息

#### Scenario: 系统权限和无效输入保护

- **WHEN** 调用方通过公开权限创建或更新接口提交系统权限标记
- **THEN** 系统 MUST 拒绝未知字段或忽略该字段并保持非系统权限语义
- **AND** 系统 MUST NOT 因调用方输入创建或篡改系统权限
- **WHEN** 权限标识、HTTP method、route template、模块或描述不满足 domain validation，或者目标权限不存在或受系统保护
- **THEN** 系统 MUST 拒绝变更并返回可区分的业务错误

#### Scenario: 路由差异诊断

- **WHEN** 系统比较已注册的可授权 HTTP 路由与权限目录
- **THEN** 系统 MUST 返回 missing、stale 和不一致的权限定义
- **AND** 诊断 MUST NOT 创建权限、修改权限状态或改变角色绑定
- **AND** 路由发现 MUST 排除 `OPTIONS`、`/api/v1/` 之外的路由以及认证公开或会话控制路由
- **AND** application 层 MUST NOT 直接依赖 Gin engine

### Requirement: 角色目录与角色权限绑定

系统 MUST 提供角色创建、更新、启停、详情、列表和角色权限绑定能力。公开写接口 MUST NOT 允许调用方创建或篡改系统角色；角色权限关系 MUST 只引用存在且启用的权限，并在完整替换时保持事务性。

#### Scenario: 角色目录写入和查询

- **WHEN** 授权调用方提交合法角色信息和可用权限集合
- **THEN** 系统 MUST 创建非系统角色并写入角色权限绑定，成功响应 MUST 返回新建角色实体
- **WHEN** 授权调用方使用合法输入更新、启用或停用存在的普通角色
- **THEN** 系统 MUST 持久化对应变更，成功响应 MUST NOT 包含更新后的角色实体
- **WHEN** 授权调用方查询角色详情或分页查询角色
- **THEN** 系统 MUST 返回角色数据、权限摘要和共享 pagination 信息

#### Scenario: 系统角色和权限绑定保护

- **WHEN** 普通角色接口尝试创建、修改或停用系统角色
- **THEN** 系统 MUST 拒绝操作并保持系统角色及其基线语义不变
- **WHEN** 角色权限写请求引用不存在或已停用的权限
- **THEN** 系统 MUST 拒绝写入并保持已有角色权限关系不变
- **AND** role application MUST 通过 permission application 拥有的最小查询端口校验权限，MUST NOT 导入 permission infrastructure

#### Scenario: 完整替换角色权限

- **WHEN** 授权调用方使用合法权限集合替换角色的完整权限绑定
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一写入发生非幂等错误时系统 MUST 回滚全部变更

#### Scenario: 停用和系统基线

- **WHEN** 角色被停用
- **THEN** 该角色 MUST NOT 出现在用户有效角色、有效权限或 Casbin policy 中
- **WHEN** seed 补齐或同步系统角色权限绑定
- **THEN** 系统 MUST 批量维护绑定并把已有绑定视为幂等成功，非唯一冲突错误 MUST 使本次事务失败

### Requirement: 用户角色绑定与有效权限

系统 MUST 支持查询、添加、移除和完整替换用户角色绑定，并基于用户当前启用角色及其启用权限提供有效权限。写路径 MUST 校验用户和角色状态，失败时 MUST 保持原绑定和同步状态不变。

#### Scenario: 用户角色绑定写入

- **WHEN** 授权调用方将存在且启用的角色绑定给存在且未软删除的用户
- **THEN** 系统 MUST 写入用户角色关系并使后续授权能够使用该角色
- **WHEN** 用户角色写请求引用不存在或已软删除的用户、不存在的角色或已停用角色
- **THEN** 系统 MUST 拒绝写入并返回明确错误
- **AND** 系统 MUST NOT 改变已有关系、失效缓存或发送 policy change 通知

#### Scenario: 完整替换用户角色

- **WHEN** 授权调用方使用全部合法且启用的角色集合替换用户的完整角色绑定
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一角色不可用或任一写入失败时系统 MUST 回滚全部变更

#### Scenario: 有效权限聚合

- **WHEN** 系统或授权调用方查询用户有效权限
- **THEN** 系统 MUST 聚合该用户当前启用角色和这些角色的启用权限并返回去重后的权限集合
- **AND** 角色、权限、用户角色和角色权限 MUST 使用外部 UUID 作为稳定业务标识，join 表内部标识 MUST NOT 暴露给 application 或 transport
- **WHEN** 已认证用户没有有效角色绑定并访问受 RBAC 保护的路由
- **THEN** 系统 MUST 拒绝访问

### Requirement: Casbin 授权与 HTTP 保护

系统 MUST 在认证通过后使用 RBAC 授权中间件保护权限、角色和用户业务接口。授权 MUST 使用用户与角色的稳定 subject、Gin route template object 和 HTTP method action，并在任何身份、策略或执行异常下 fail-closed。

#### Scenario: 构造和执行授权请求

- **WHEN** 已认证请求进入受 RBAC 保护的 `/api/v1` 路由
- **THEN** 中间件 MUST 使用请求上下文中的用户 ID、Gin `FullPath()` 和 HTTP method 构造授权请求
- **AND** 用户 subject MUST 使用 `user:<user_uuid>`，角色 subject MUST 使用 `role:<role_uuid>`
- **WHEN** 用户当前启用角色拥有匹配 HTTP method 和 route template 的启用权限
- **THEN** 系统 MUST 允许请求进入目标 controller
- **AND** 没有匹配权限时系统 MUST 返回禁止访问错误

#### Scenario: fail-closed 授权边界

- **WHEN** 请求缺少用户 ID、用户 ID 类型非法或 subject 不能解析为用户 UUID
- **THEN** 系统 MUST 返回未认证错误并拒绝请求，且 MUST NOT 调用底层 Casbin engine
- **WHEN** 用户角色回源失败、context 取消、Casbin 执行返回错误、policy 未加载或最近一次 reload 失败
- **THEN** 系统 MUST 拒绝受保护请求并暴露 policy 不可用 readiness/startup 状态
- **AND** 系统 MUST NOT 将执行异常或 policy 缺失折叠为允许结果

#### Scenario: 路由旁路和注册安全

- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS` 方法
- **THEN** 中间件 MUST 允许请求继续处理并 MUST NOT 调用授权服务
- **WHEN** user-service 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 授权中间件链
- **AND** token version validator、RBAC authorizer 或必需 route registrar 缺失时系统 MUST 拒绝注册部分业务路由

#### Scenario: policy 权威来源和超级管理员

- **WHEN** policy loader 构造授权策略
- **THEN** policy MUST 从启用角色、启用权限、角色权限绑定和用户角色绑定派生
- **AND** 独立 `casbin_rules` 表 MUST NOT 成为业务权威来源，用户身份解析 MUST 排除已软删除用户
- **WHEN** 用户拥有 `internal/shared/rbacbaseline` 定义的内置超级管理员角色
- **THEN** policy loader MUST 为该角色提供受保护业务接口的 wildcard policy
- **AND** 超级管理员角色常量 MUST 只由 `rbacbaseline` 提供

### Requirement: RBAC 策略加载、缓存与多副本同步

系统 MUST 以 PostgreSQL 关系数据作为业务权威来源，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。系统 MUST 在启动时显式加载 policy，在线 RBAC 写操作成功后刷新本实例状态，并通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本。授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

#### Scenario: 初始加载和恢复

- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 显式调用初始 policy 加载入口，并将可取消或带超时的启动 context 传给 policy loader
- **WHEN** 初始 policy 加载失败或被取消
- **THEN** engine MUST 记录最近错误和 reload 失败指标，后续授权 MUST fail-closed，`app.Start` MUST 保持成功
- **AND** reload 状态和 readiness/startup 可观测性 MUST 保留失败信息并拒绝接入业务流量
- **WHEN** 后续 Pub/Sub、版本补偿或显式 reload 成功加载当前 policy
- **THEN** engine MUST 替换为最新可用 policy、清除最近 reload 错误并恢复 readiness/startup

#### Scenario: 用户角色缓存语义

- **WHEN** 用户角色缓存命中
- **THEN** 授权 MUST 使用缓存中的角色 ID 副本，调用方对返回 slice 的修改 MUST NOT 污染缓存内部值
- **WHEN** 用户角色缓存未命中
- **THEN** 系统 MUST 合并同一用户的并发回源并查询 PostgreSQL 中的当前启用角色，loader 错误 MUST NOT 写入缓存
- **WHEN** `rbac.user_role_cache.enabled` 为 false
- **THEN** 系统 MUST 直接回源并保持正确的 fail-closed 授权语义

#### Scenario: 在线写后同步

- **WHEN** 权限、角色状态、角色权限绑定或用户角色绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST reload policy 或失效相关用户缓存，并发布新的 Redis policy version 和 Pub/Sub 通知
- **AND** 持久化成功后的 reload、缓存失效、version 发布或通知失败 MUST 向调用方返回同步错误，成功响应 MUST NOT 掩盖该错误
- **AND** `PolicyChangeNotifier` MUST 是正式 command service 的必需依赖

#### Scenario: reload、发布和副本收敛

- **WHEN** policy refresh coordinator 执行本地 reload 和 version 发布
- **THEN** 本地 reload 失败后系统 MUST 仍尝试发布 version
- **AND** 两者同时失败时返回错误 MUST 保留两项失败，只有两者均成功时系统才 MUST 标记本实例已应用该 version
- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现远端 policy version 更新
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛

### Requirement: RBAC 系统数据与运维 CLI

系统 MUST 提供带服务上下文的 `rbac seed`、`rbac assign-super-admin` 和 `rbac create-super-admin` 命令，用于维护系统角色、系统权限、默认绑定和超级管理员。系统数据 MUST 只由 seed port 根据 `internal/shared/rbacbaseline` 写入。

#### Scenario: 初始化系统基线

- **WHEN** 运维执行 `aegiscore-user-service rbac seed`
- **THEN** 系统 MUST 幂等创建或更新基线角色、权限和绑定并输出变更统计
- **AND** 系统角色和权限 MUST 标记为系统数据
- **AND** seed MUST NOT 创建业务用户或自动分配超级管理员
- **AND** 非 seed 的角色或权限 command、store create 或公开 HTTP 路径 MUST 固定写入非系统数据并 MUST NOT 接收系统标记

#### Scenario: 超级管理员维护

- **WHEN** 运维执行 `rbac assign-super-admin --user-id <uuid>`
- **THEN** 系统 MUST 为指定存在用户幂等绑定内置超级管理员角色
- **WHEN** 运维执行 `rbac create-super-admin` 并提供合法 username 和密码
- **THEN** 系统 MUST 创建或复用用户并绑定内置超级管理员角色
- **AND** username MUST trim 后转为小写，空 nickname MUST 回退为归一化 username
- **AND** 未显式指定 password env 时系统 MUST 从 `ADMIN_PASSWORD` 读取非空密码
- **AND** 已有用户的密码 MUST NOT 默认重置，只有显式 `--reset-password` 或 `ADMIN_RESET_PASSWORD=true` 时系统才 MUST 更新密码
- **AND** 必需输入缺失时命令 MUST 返回明确错误

#### Scenario: 离线命令不等同在线刷新

- **WHEN** HTTP 副本运行期间执行 seed、assign-super-admin 或 create-super-admin
- **THEN** 命令 MUST 只修改持久化数据并 MUST NOT 宣称已触发运行期 policy refresh
- **AND** 运维 MUST 滚动重启副本或触发在线 RBAC 刷新使运行实例收敛

### Requirement: RBAC 错误与统一 HTTP 契约

permission、role 和 binding domain MUST 返回携带稳定 HTTP status、共享业务 code、公开 message 和 `Reason` 的应用错误。HTTP transport MUST 通过共享 `response.Fail` 直接渲染业务错误，MUST NOT 维护 feature 专用 sentinel-to-HTTP mapper；直接或包装返回的应用错误 MUST 保留 `errors.Is` 语义。

#### Scenario: 目录与绑定错误稳定映射

- **WHEN** permission feature 返回权限已存在、权限不存在、输入无效或系统权限保护错误
- **THEN** 系统 MUST 分别使用 `409 Conflict`、`404 Not Found`、`400 Bad Request`、`409 Conflict`，且 `Reason` MUST 分别为 `permission_already_exists`、`permission_not_found`、`permission_invalid`、`system_permission_protected`
- **WHEN** role feature 返回角色已存在、角色不存在、输入无效、系统角色保护或角色停用错误
- **THEN** 系统 MUST 分别使用 `409 Conflict`、`404 Not Found`、`400 Bad Request`、`409 Conflict`、`409 Conflict`，且 `Reason` MUST 分别为 `role_already_exists`、`role_not_found`、`role_invalid`、`system_role_protected`、`role_inactive`
- **WHEN** 用户角色或角色权限增量绑定返回绑定已存在或绑定不存在错误
- **THEN** 系统 MUST 分别返回 `409 Conflict` 或 `404 Not Found`，并使用对应稳定 `Reason`

#### Scenario: 跨 feature 错误透传和统一出口

- **WHEN** role 流程收到 `identity.ErrUserNotFound` 或 permission 的不存在错误
- **THEN** role HTTP transport MUST 通过共享 response helper 保留错误自身的 status、code 和 message
- **AND** role transport MUST NOT 复制 identity 或 permission 错误映射
- **WHEN** permission 或 role controller 的 command/query 返回业务错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** transport MUST NOT 调用或保留 `toPermissionHTTPError`、`toRoleHTTPError` 或等价 mapper

### Requirement: RBAC 可观测性

系统 MUST 为 RBAC 授权判定和正式模块执行的 route diff 提供低基数 metrics，并使用显式注入的 logger 记录加载和同步异常。观测失败 MUST NOT 改变授权或策略同步结果。

#### Scenario: 授权 metrics 的低基数与敏感数据约束

- **WHEN** permission authorization service 完成一次 RBAC Enforce 判定
- **THEN** counter MUST 记录 `result="allow"`、`result="deny"` 或 `result="error"`
- **AND** histogram MUST 记录本次判定耗时
- **AND** 标签 MUST 只使用 `result`、HTTP method 和 route template
- **AND** 指标 MUST NOT 包含用户、角色、权限、token、trace、IP、账号、Redis key、SQL、原始错误或 raw path

#### Scenario: route diff 和日志观测

- **WHEN** 正式 permission 模块执行 route diff
- **THEN** 系统 MUST 记录本次 missing、stale 和不一致结果
- **AND** 指标记录 MUST NOT 修改权限目录或路由诊断结果
- **WHEN** role 或 permission application、policy loader、watcher、cache 或 adapter 需要记录日志
- **THEN** logger MUST 由 constructor 显式注入或由调用方 context 提供
- **AND** 日志 MUST 使用稳定低基数字段并 MUST NOT 记录 token、SQL、Redis key 或原始 policy 数据

### Requirement: RBAC 架构、装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应 composition、transport 或 infrastructure 边界。RBAC 自有 watcher、cache 和 policy 投影资源 MUST 显式启动、停止和回滚。

#### Scenario: 分层和最小依赖

- **WHEN** role 或 permission application service 在单元测试或非 Fx 调用方中构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明仅服务于 DI 的 tag
- **AND** 消费侧 application MUST 定义最小 port 并由相邻 feature 或 integration adapter 实现，feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport

#### Scenario: framework-neutral adapter 和 composition 边界

- **WHEN** 构造 role store、permission store、policy loader、Casbin engine、Redis policy store、watcher、cache 或 adapter
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options
- **AND** constructor MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、named result 或 group result
- **AND** 具名 `primary_db`、`cache_redis`、optional、group 或生命周期依赖选择 MUST 留在 feature composition 边界
- **AND** public provider MUST 只暴露 controller、authorizer、route registrar、health/status 和 application port 等稳定 contract，父 module MUST NOT 消费 feature infrastructure concrete implementation

#### Scenario: 有状态资源单实例多视图

- **WHEN** composition 需要同时提供 authorization、policy reload、policy health、policy store 或 publisher 等接口视图
- **THEN** composition MUST 为同一有状态 adapter 构造一个实例并通过普通 Go 赋值暴露所需端口
- **AND** 系统 MUST NOT 为不同接口视图重复构造有状态 engine、store、version tracker、watcher 或 cache

#### Scenario: 必需同步依赖不可降级

- **WHEN** 权限、角色、角色权限或用户角色写侧服务装配完成
- **THEN** 服务 MUST 具备可用的 policy change notifier
- **AND** 缺少 notifier 或其他必需安全 collaborator 时 constructor MUST 返回明确 error 并拒绝装配，MUST NOT panic
- **AND** 系统 MUST NOT 以 no-op、nil fallback 或兼容 wrapper 静默跳过 policy reload、Redis policy version 或 watcher 同步语义

#### Scenario: watcher、cache 和 lifecycle

- **WHEN** user-service 启停 Redis policy watcher 和 user-role cache
- **THEN** `NewWatcher` MUST 只构造 watcher 对象，MUST NOT 启动 goroutine、订阅 Redis 或执行版本补偿循环
- **AND** `Start()` 和 `Stop(ctx)` MUST 幂等，`Stop(ctx)` MUST 取消内部 context，并在调用方 context 限制内等待循环退出
- **AND** `Stop(ctx)` 超时时 MUST 返回 context 相关错误，并保持后续重复停止安全
- **AND** 启动失败或服务停止时已启动 watcher MUST 被停止，已创建 cache MUST 幂等关闭
- **AND** watcher stop 和 cache close 同时失败时单个 lifecycle hook MUST 保留全部 cause，且 cache close MUST 在 watcher stop 返回错误时仍被执行

#### Scenario: 共享资源所有权和 fail-closed

- **WHEN** RBAC 关闭 watcher、cache、store 或 resolver
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源
- **AND** 关闭后授权语义 MUST 继续 fail-closed，不得因为本地资源不可用而产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置或 key schema 下沉到 `common`
