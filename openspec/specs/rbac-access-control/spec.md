## Purpose

定义 user-service 的 RBAC 访问控制能力，覆盖权限目录、角色、角色权限、用户角色、Casbin 授权、系统 seed 和超级管理员引导。
## Requirements
### Requirement: 权限目录管理

系统 MUST 提供权限目录创建、更新、启停、查询、列表和路由差异分析能力，用于描述可授权的 HTTP 资源和动作。权限创建 MUST 返回新建权限实体；权限更新、启用和停用成功后 MUST 返回无实体成功响应，调用方如需最新实体 MUST 使用查询接口读取。

#### Scenario: 创建权限

- **WHEN** 授权调用方提供合法权限标识、方法、路径和描述
- **THEN** 系统 MUST 创建权限记录，并使其可参与后续角色绑定和授权判断
- **AND** 系统 MUST 返回新建权限实体

#### Scenario: 更新权限不返回实体

- **WHEN** 授权调用方更新存在的权限目录记录且输入合法
- **THEN** 系统 MUST 持久化权限元数据变更
- **AND** 成功响应 MUST NOT 包含权限实体响应体
- **AND** 持久化层 MUST NOT 为构造成功响应而在更新后重新查询该权限实体

#### Scenario: 启停权限不返回实体

- **WHEN** 授权调用方启用或停用存在的权限目录记录
- **THEN** 系统 MUST 持久化权限启用状态变更
- **AND** 成功响应 MUST NOT 包含权限实体响应体
- **AND** 持久化层 MUST NOT 为构造成功响应而在更新后重新查询该权限实体

#### Scenario: 权限输入非法

- **WHEN** 权限方法、路径、标识或描述不满足 domain validation
- **THEN** 系统 MUST 拒绝创建或更新，并返回一致的校验错误

#### Scenario: 路由差异分析

- **WHEN** 系统扫描已注册 HTTP 路由并与权限目录比较
- **THEN** 系统 MUST 能识别 missing、stale 或不一致的权限定义，且 MUST NOT 创建权限、修改权限状态或绑定角色

#### Scenario: 可授权路由发现

- **WHEN** 系统构造 route diff 诊断输入
- **THEN** 系统 MUST 排除 `OPTIONS`、`/api/v1/` 外路径和认证公开或会话控制路由，且 application 层 MUST NOT 直接依赖 Gin engine

### Requirement: 角色与权限绑定

系统 MUST 提供角色创建、更新、查询、列表和角色权限绑定能力，并保证绑定引用的权限存在且状态可用。角色创建 MUST 返回新建角色实体；角色更新、启用和停用成功后 MUST 返回无实体成功响应。角色权限绑定的替换、系统绑定补齐和系统绑定同步 MUST 使用批量写入方式新增多条绑定，并保持事务性和错误语义。

#### Scenario: 创建角色并绑定权限

- **WHEN** 授权调用方创建角色并指定合法权限集合
- **THEN** 系统 MUST 持久化角色、写入角色权限绑定，并使授权策略可同步使用

#### Scenario: 更新角色不返回实体

- **WHEN** 授权调用方更新存在的角色记录且输入合法
- **THEN** 系统 MUST 持久化角色元数据变更
- **AND** 成功响应 MUST NOT 包含角色实体响应体
- **AND** 持久化层 MUST NOT 为构造成功响应而在更新后重新查询该角色实体

#### Scenario: 启停角色不返回实体

- **WHEN** 授权调用方启用或停用存在的角色记录
- **THEN** 系统 MUST 持久化角色启用状态变更
- **AND** 成功响应 MUST NOT 包含角色实体响应体
- **AND** 持久化层 MUST NOT 为构造成功响应而在更新后重新查询该角色实体

#### Scenario: 绑定不存在权限

- **WHEN** 角色绑定请求引用不存在或不可用的权限
- **THEN** 系统 MUST 拒绝绑定并保持已有角色权限关系不被破坏

#### Scenario: 角色通过权限端口校验

- **WHEN** 角色 application 需要校验权限 ID
- **THEN** 角色 feature MUST 通过权限 application 查询端口校验，不得导入 permission infrastructure

#### Scenario: 停用角色

- **WHEN** 角色已停用
- **THEN** 通过该角色形成的绑定 MUST NOT 在有效权限查询或 Casbin policy 加载中授予访问权限

#### Scenario: 查询角色列表

- **WHEN** 授权调用方分页查询角色
- **THEN** 系统 MUST 返回角色列表、权限摘要和共享 pagination 信息

#### Scenario: 批量替换角色权限绑定

- **WHEN** 授权调用方使用合法权限集合替换角色的完整权限绑定
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一新增绑定发生非幂等错误时，系统 MUST 回滚本次删除和新增

#### Scenario: 批量维护系统角色权限绑定

- **WHEN** RBAC seed 补齐或同步系统角色权限绑定
- **THEN** 系统 MUST 批量新增缺失绑定
- **AND** 已存在绑定的唯一冲突 MUST 保持幂等成功语义
- **AND** 非唯一冲突错误 MUST 使本次操作失败并保持既有事务回滚语义

### Requirement: RBAC 查询索引支撑

系统 MUST 为 RBAC 角色、权限、用户角色绑定、角色权限绑定和授权策略加载维护与稳定访问路径匹配的数据库索引，并通过 Ent schema 和 Atlas migration 交付可审查的结构变更。

#### Scenario: 角色列表和授权回源索引

- **WHEN** 系统分页查询角色、按启用状态过滤角色或在授权热路径回源查询用户启用角色
- **THEN** 角色表 MUST 提供支持过滤字段和 `role_id` 稳定排序的索引

#### Scenario: 权限列表索引

- **WHEN** 系统分页查询权限并按模块、HTTP 方法、启用状态或系统权限标记过滤
- **THEN** 权限表 MUST 提供支持常用过滤字段和 `permission_id` keyset 排序的索引

#### Scenario: 用户角色绑定反向索引

- **WHEN** 系统从角色侧 join 或反查用户角色绑定
- **THEN** 用户角色绑定表 MUST 提供以 `role_id` 起始并包含 `user_id` 的索引

#### Scenario: 角色权限绑定反向索引

- **WHEN** 系统从权限侧 join 或反查角色权限绑定
- **THEN** 角色权限绑定表 MUST 提供以 `permission_id` 起始并包含 `role_id` 的索引

#### Scenario: RBAC 索引不改变授权语义

- **WHEN** RBAC 查询索引发生调整
- **THEN** 权限目录、角色绑定、用户角色绑定、有效权限聚合、Casbin policy loader、policy sync 和超级管理员通配授权的业务结果 MUST 保持不变

### Requirement: 用户角色绑定

系统 MUST 支持将启用角色绑定到用户，并为授权判断提供用户有效权限查询能力。用户角色替换 MUST 使用批量写入方式新增多条绑定，并保持事务性和错误语义。用户角色绑定写路径 MUST 拒绝不存在或已停用的角色，且任一角色不可绑定时 MUST 不写入新的用户角色关系。

#### Scenario: 绑定角色给用户

- **WHEN** 授权调用方把存在且启用的角色绑定给存在用户
- **THEN** 系统 MUST 写入用户角色关系，并使该用户后续访问权限生效

#### Scenario: 用户或角色不存在

- **WHEN** 用户角色绑定请求引用不存在的用户或角色
- **THEN** 系统 MUST 拒绝绑定并返回明确错误

#### Scenario: 绑定停用角色

- **WHEN** 用户角色绑定请求引用已停用角色
- **THEN** 系统 MUST 拒绝绑定并返回明确错误
- **AND** 系统 MUST NOT 写入新的用户角色关系
- **AND** 系统 MUST NOT 触发用户角色缓存失效或 policy change 通知

#### Scenario: 查询用户有效权限

- **WHEN** 系统或调用方查询某用户有效权限
- **THEN** 系统 MUST 聚合用户角色和角色权限，返回该用户当前可访问的权限集合

#### Scenario: 用户无角色

- **WHEN** 已认证用户没有有效角色绑定并访问 RBAC 保护路由
- **THEN** 系统 MUST 拒绝访问

#### Scenario: 批量替换用户角色绑定

- **WHEN** 授权调用方使用合法角色集合替换用户的完整角色绑定
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一新增绑定失败时，系统 MUST 回滚本次删除和新增

#### Scenario: 批量替换包含停用角色

- **WHEN** 授权调用方替换用户角色集合且任一目标角色已停用
- **THEN** 系统 MUST 拒绝本次替换并返回明确错误
- **AND** 系统 MUST 保持该用户已有角色关系不变
- **AND** 系统 MUST NOT 触发用户角色缓存失效或 policy change 通知

### Requirement: 授权热路径用户角色本地缓存

系统 MUST 在 RBAC 授权热路径中使用有容量上限的本地 loading cache 缓存用户当前启用角色 ID 集合，并通过主动失效和全量清空保证在线 RBAC 变更后不依赖 TTL 长期收敛。user-service permission/RBAC provider 边界 MUST 拥有 `rbac_user_roles` 缓存实例名，并 MUST 在缺少该配置实例时拒绝服务装配。

#### Scenario: 用户角色缓存命中

- **WHEN** 业务请求进入 RBAC 授权中间件且用户角色本地缓存命中
- **THEN** 授权判断 MUST 使用缓存中的角色 ID 副本
- **AND** 调用方对返回 slice 的修改 MUST NOT 污染缓存内部值

#### Scenario: 用户角色缓存 miss

- **WHEN** 业务请求进入 RBAC 授权中间件且用户角色本地缓存 miss
- **THEN** 系统 MUST 通过 `singleflight` 合并同用户并发回源
- **AND** 回源 MUST 查询 PostgreSQL 中该用户当前绑定的启用角色
- **AND** loader 错误 MUST NOT 写入本地缓存

#### Scenario: 用户角色缓存容量边界

- **WHEN** 单实例处理大量不同用户的 RBAC 授权请求
- **THEN** `rbac_user_roles` 本地缓存 MUST 使用配置容量限制进程内条目预算
- **AND** 容量淘汰、准入拒绝或 TTL 过期后 MUST 能通过 PostgreSQL 回源恢复授权判断

#### Scenario: 用户角色必需缓存配置

- **WHEN** user-service 装配 RBAC 用户角色 resolver
- **THEN** permission/RBAC provider MUST 使用本服务常量读取 `local_cache.rbac_user_roles`
- **AND** 缺少该配置实例时 MUST 返回明确错误并拒绝继续装配用户角色本地缓存

#### Scenario: 在线用户角色变更失效缓存

- **WHEN** 用户角色绑定通过在线 HTTP API 添加、替换或移除成功
- **THEN** 本实例 MUST 删除对应用户角色本地缓存或清空相关缓存
- **AND** 其他副本 MUST 通过既有 policy sync 机制感知变更并失效本地缓存

#### Scenario: policy reload 全量失效缓存

- **WHEN** RBAC policy reload 或全量策略刷新完成
- **THEN** 系统 MUST 清空本实例用户角色本地缓存
- **AND** 后续授权请求 MUST 通过回源重新建立本地投影

### Requirement: Casbin 授权保护

系统 MUST 使用 RBAC 授权中间件保护权限、角色和用户业务接口，并在认证通过后执行资源级授权判断。Casbin subject/object/action MUST 分别使用 `user:<user_uuid>`、`role:<role_uuid>`、Gin route template 和 HTTP method。授权服务 MUST 区分认证 subject 非法与策略拒绝，且在 subject 非法时 MUST 拒绝请求并返回明确错误，不得将解析失败静默折叠为普通权限拒绝。系统 MUST 将 user-service 直接依赖的 Casbin 主版本维护在最新稳定 v3，并保持 RBAC 授权、policy loader、超级管理员通配授权、用户角色缓存和 policy sync 的既有业务语义不变。升级后系统 MUST NOT 保留对 `github.com/casbin/casbin/v2` 或其子包的直接引用。

#### Scenario: 授权通过

- **WHEN** 已认证用户拥有当前 HTTP 方法和路径对应权限
- **THEN** 系统 MUST 允许请求进入目标 controller

#### Scenario: 授权失败

- **WHEN** 已认证用户缺少当前 HTTP 方法和路径对应权限
- **THEN** 系统 MUST 拒绝请求并返回授权失败错误

#### Scenario: 非法认证 subject

- **WHEN** 授权服务收到无法解析为用户 UUID 的认证 subject
- **THEN** 系统 MUST 拒绝请求并返回明确错误
- **AND** 系统 MUST NOT 调用底层授权 engine
- **AND** 调用方 MUST 能通过错误区分该场景与普通策略拒绝

#### Scenario: 权限策略更新

- **WHEN** 权限、角色或绑定发生变化
- **THEN** 系统 MUST 同步或刷新授权策略，避免旧策略长期影响授权判断

#### Scenario: Casbin policy 权威来源

- **WHEN** policy loader 从持久化层构造授权策略
- **THEN** 策略 MUST 由启用角色、启用权限、角色权限绑定和用户角色绑定派生，不得以独立 `casbin_rules` 表作为业务权威来源

#### Scenario: Casbin subject 稳定格式

- **WHEN** 角色参与 policy 构造或授权判断
- **THEN** 角色 subject MUST 使用 `role:<role_uuid>`，不得依赖 `roles.code`；用户身份解析 MUST 排除已软删除用户

#### Scenario: 超级管理员通配授权

- **WHEN** 用户拥有 `internal/shared/rbacbaseline` 中稳定的内置超级管理员角色
- **THEN** policy loader MUST 补充 wildcard policy，使其可访问受保护业务接口，且升级后的 Casbin v3 enforcer MUST 继续识别 wildcard policy
- **AND** 超级管理员角色常量 MUST 仍只由 `internal/shared/rbacbaseline` 提供，且 MUST NOT 在 role 或 permission feature 内重复定义超级管理员常量

#### Scenario: 使用稳定 v3 模块路径

- **WHEN** 实现升级 Casbin 依赖
- **THEN** `user-service/go.mod` MUST 直接依赖 `github.com/casbin/casbin/v3` 的最新稳定版本
- **AND** user-service 代码和测试 MUST NOT import `github.com/casbin/casbin/v2` 或 `github.com/casbin/casbin/v2/model`

#### Scenario: 授权判断语义保持不变

- **WHEN** 已认证用户访问 RBAC 保护路由
- **THEN** 授权判断 MUST 继续使用用户当前启用角色、`role:<role_uuid>` subject、Gin route template 和 HTTP method 执行 Casbin `Enforce`
- **AND** policy 未加载、用户无启用角色、角色无匹配权限或底层 Casbin 返回错误时 MUST NOT 默认放行

### Requirement: Casbin v3 API 变更检查与适配记录

实现 MUST 全面检查当前代码中所有 Casbin 旧用法，并对模块路径、model 子包、enforcer 构造、policy 写入、授权执行和测试 helper 给出明确适配。检查结果 MUST 记录在实现任务或提交说明中，并通过编译、测试和全仓搜索验证没有遗漏旧主版本引用。

#### Scenario: 旧用法替换完整

- **WHEN** 实现完成 Casbin v3 升级
- **THEN** `rg "github.com/casbin/casbin/v2|casbin/v2" common user-service tools --glob '*.go' --glob 'go.mod' --glob 'go.sum'` MUST 无命中
- **AND** 所有直接 Casbin API 调用 MUST 使用 v3 模块下的类型或函数

#### Scenario: 关键 API 行为验证

- **WHEN** 实现适配 `NewEnforcer`、`AddPolicy`、`Enforce` 和 `model` import
- **THEN** 测试 MUST 覆盖允许、拒绝、底层错误、未配置、通配策略、context 取消和 policy reload 失败路径
- **AND** 失败路径 MUST 保留当前错误区分和错误包装语义

#### Scenario: v3 新能力不改变线上授权路径

- **WHEN** 实现评估 Casbin v3 新增的 `Explain()`、detector 或其他诊断能力
- **THEN** 这些能力 MUST NOT 成为 request-time 授权 allow/deny 的必要依赖
- **AND** 如需采用这些能力优化诊断，MUST 仅作为离线 route diff、管理端排障或后续独立 change 的候选方案记录

### Requirement: Casbin 初始 policy 加载上下文传播

系统 MUST 在 user-service 启动 lifecycle 中执行 Casbin Engine 初始 policy 加载，并将 Fx `OnStart` 提供的启动 context 传播到 policy loader 及其持久化查询。Casbin Engine 构造器 MUST NOT 在 provider 构造阶段执行不可取消的初始 policy reload。

#### Scenario: 启动 context 传播到初始加载

- **WHEN** user-service 通过 Fx 启动 permission/RBAC 模块并初始化 Casbin Engine
- **THEN** 初始 policy reload MUST 使用 Fx `OnStart` 传入的 context 调用 `Loader.LoadPolicies(ctx)`
- **AND** 启动 context 取消或超时时，底层 policy loader MUST 能观察到对应取消信号

#### Scenario: 初始加载失败保持 fail-closed

- **WHEN** Casbin 初始 policy 加载失败或因启动 context 取消而未构造可用 enforcer
- **THEN** Engine MUST 记录最近错误并更新 policy reload 失败指标
- **AND** 后续授权判断 MUST fail-closed，不得放行缺少可用 policy 的请求
- **AND** 服务装配行为 MUST 保持既有语义，不因本场景自动改为启动失败

#### Scenario: 手动 reload 继续使用调用方 context

- **WHEN** 在线 RBAC 写操作或 watcher 触发 Casbin policy reload
- **THEN** `Reload(ctx)` MUST 继续使用调用方传入的 context 执行 policy loader
- **AND** 本次变更不得改变 policy 权威来源、用户角色缓存失效或 Redis policy sync 语义

### Requirement: RBAC policy watcher 生命周期契约

RBAC Redis policy watcher MUST 使用无参数 `Start()` 表达启动动作，并 MUST 通过内部可取消 context 驱动长期后台循环；Fx `OnStart` context MUST NOT 作为 watcher 后台循环的生命周期控制信号。watcher 停止 MUST 由 `Stop(ctx)` 触发内部 cancel 并等待后台循环退出，`Stop(ctx)` 的 context 仅用于限制停止等待时间。

#### Scenario: 启动 watcher 不消费 Fx 启动 context

- **WHEN** user-service 通过 Fx `OnStart` 启动 RBAC Redis policy watcher
- **THEN** lifecycle hook MUST 调用无参数 `Watcher.Start()`
- **AND** watcher 后台循环 MUST NOT 依赖 Fx `OnStart` context 的取消信号来退出

#### Scenario: Stop 关闭 watcher 后台循环

- **WHEN** user-service 通过 Fx `OnStop` 停止 RBAC Redis policy watcher
- **THEN** `Watcher.Stop(ctx)` MUST 取消 watcher 内部 context 并等待后台循环退出
- **AND** `Stop(ctx)` MUST 在传入 context 取消或超时时返回对应错误

#### Scenario: 策略同步语义保持不变

- **WHEN** watcher 通过 Redis Pub/Sub 或周期性版本补偿感知 RBAC policy version 变化
- **THEN** 系统 MUST 继续按既有 policy sync 语义执行 policy reload 或用户角色缓存失效
- **AND** 本生命周期契约变更 MUST NOT 改变 Redis policy version、Pub/Sub payload、补偿检查间隔、Casbin reload 或授权判断语义

### Requirement: Permission HTTP 授权中间件请求构造与旁路行为

permission HTTP 授权中间件 MUST 在真实 Gin 路由上下文中解析授权请求，并将认证用户 ID、Gin route template 和 HTTP method 传递给 permission authorization service；白名单和 `OPTIONS` 请求 MUST 绕过授权服务调用。

#### Scenario: 授权请求使用 Gin route template

- **WHEN** 已认证用户访问受 RBAC 保护的 permission HTTP 路由
- **THEN** 授权中间件 MUST 使用认证用户 ID 作为授权 subject
- **AND** 授权中间件 MUST 使用 Gin `FullPath()` route template 作为授权 object
- **AND** 授权中间件 MUST 使用 HTTP method 作为授权 action

#### Scenario: 认证用户来自请求上下文

- **WHEN** 已认证用户 ID 存在于 request context 且 Gin context 未设置用户 ID
- **THEN** 授权中间件 MUST 使用 request context 中的用户 ID 构造授权请求

#### Scenario: 缺失或非法用户不调用授权服务

- **WHEN** 请求缺少认证用户 ID 或 Gin context 中的用户 ID 类型非法
- **THEN** 授权中间件 MUST 拒绝请求并返回未认证错误
- **AND** 授权中间件 MUST NOT 调用 permission authorization service

#### Scenario: 白名单请求绕过授权服务

- **WHEN** 请求方法和 Gin route template 命中显式授权白名单
- **THEN** 授权中间件 MUST 允许请求继续处理
- **AND** 授权中间件 MUST NOT 调用 permission authorization service

#### Scenario: OPTIONS 请求绕过授权服务

- **WHEN** 请求使用 `OPTIONS` 方法访问已注册路由
- **THEN** 授权中间件 MUST 允许请求继续处理
- **AND** 授权中间件 MUST NOT 调用 permission authorization service

#### Scenario: 授权服务拒绝或错误映射响应

- **WHEN** permission authorization service 返回拒绝、执行错误或 invalid subject 错误
- **THEN** 授权中间件 MUST 分别返回禁止访问、内部错误或未认证错误响应

### Requirement: 策略同步

系统 MUST 在在线 RBAC 写操作成功后触发本实例策略刷新，并通过 Redis policy version、Pub/Sub 和定时版本补偿同步其他副本。写操作成功响应是否包含实体 MUST NOT 影响 policy reload、用户角色缓存失效或跨副本同步语义。在线 RBAC 写操作持久化成功后，policy reload、用户角色缓存失效或 Redis policy version 发布失败 MUST 向调用方返回错误，MUST NOT 被成功响应掩盖。

#### Scenario: 在线角色绑定变更

- **WHEN** 用户角色绑定通过 HTTP API 变更成功
- **THEN** 本实例 MUST 执行策略刷新或用户角色缓存失效，并通知其他副本

#### Scenario: 在线权限策略变更

- **WHEN** 在线 RBAC 管理接口修改权限、角色启停或角色权限绑定并提交成功
- **THEN** 本实例 MUST 执行 policy reload，并通过 Redis policy version 和 Pub/Sub 通知其他副本；其他副本 MUST 通过 Pub/Sub 和周期性版本补偿感知变更

#### Scenario: 授权热路径

- **WHEN** 业务请求进入 RBAC 授权中间件
- **THEN** 授权 MUST 使用本实例内存 Casbin enforcer 和本地可用的用户角色解析结果，MUST NOT 每请求读取 Redis policy version 做强一致门控

#### Scenario: 写响应契约不影响同步

- **WHEN** 权限、角色、用户角色绑定或角色权限绑定写操作成功且响应不包含更新后实体
- **THEN** 系统 MUST 继续按既有规则触发本实例 policy reload、用户角色缓存失效和跨副本 policy version 通知

#### Scenario: policy change 通知失败向调用方传播

- **WHEN** 权限、角色、用户角色绑定或角色权限绑定写操作已经持久化成功，但 policy change 通知、policy reload、用户角色缓存失效或 Redis policy version 发布返回错误
- **THEN** command service MUST 向调用方返回该错误
- **AND** 成功响应 MUST NOT 掩盖同步失败
- **AND** 错误链 SHOULD 保留底层同步错误，便于日志、metrics 和测试定位失败来源

#### Scenario: policy change notifier 必需依赖

- **WHEN** permission command service 或 role command service 被构造
- **THEN** `PolicyChangeNotifier` MUST 作为必需依赖提供
- **AND** 缺失 notifier 时系统 MUST fail-fast 或拒绝装配，MUST NOT 退化为 no-op 通知

#### Scenario: policy refresh coordinator 局部失败处理

- **WHEN** policy refresh coordinator 收到需要本实例 reload 并发布 policy version 的变更
- **THEN** coordinator 为 nil 时 MUST 返回明确错误
- **AND** 本地 reload 失败后仍 MUST 尝试发布 policy version，使其他副本有机会感知变更
- **AND** reload 和 publish 同时失败时 MUST 通过 joined error 保留两者
- **AND** 只有本地 reload 成功且 publish 成功时才可标记本实例已应用该 policy version

### Requirement: RBAC 系统数据引导

系统 MUST 提供 CLI 能力初始化系统角色、系统权限、系统绑定，并支持为用户分配或创建超级管理员。

#### Scenario: 初始化 RBAC 系统数据

- **WHEN** 运维执行 `aegiscore-user-services rbac seed`
- **THEN** 系统 MUST 创建或更新默认系统角色、权限和绑定，并输出插入、更新、绑定增删统计；seed MUST NOT 自动创建真实业务用户或为任意业务用户分配超级管理员角色

#### Scenario: 分配超级管理员

- **WHEN** 运维执行 `rbac assign-super-admin --user-id <uuid>`
- **THEN** 系统 MUST 为指定已存在用户绑定内置超级管理员角色

#### Scenario: 创建超级管理员

- **WHEN** 运维执行 `create-super-admin` 并提供管理员密码环境变量
- **THEN** 系统 MUST 创建或复用管理员用户并绑定内置超级管理员角色；已有管理员默认 MUST NOT 重置密码，只有显式传入 `--reset-password` 或 `ADMIN_RESET_PASSWORD=true` 时才允许重置密码

#### Scenario: 缺少管理员密码

- **WHEN** 创建超级管理员时缺少配置的密码环境变量或密码为空
- **THEN** 系统 MUST 拒绝执行并返回明确错误

#### Scenario: 运行中执行离线 RBAC 命令

- **WHEN** HTTP 副本已经运行时执行 `rbac seed`、`rbac assign-super-admin` 或 `rbac create-super-admin`
- **THEN** 命令只修改持久化数据，不得被视为运行期 policy refresh；运维 MUST 滚动重启副本或触发在线 RBAC 刷新

### Requirement: role command service 测试协作者契约

role command service 测试 MUST 使用 `user-service/internal/features/role/application/command` 包已有 `mockgen` 生成物表达 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup` 和 `PolicyChangeNotifier` 等外部协作者契约。测试 MUST NOT 保留实现这些 port 的手写 store/notifier double 来兼容或隐藏依赖调用、失败路径、调用顺序、去重逻辑或 policy change 通知行为。

#### Scenario: 角色生命周期测试使用生成 mock

- **WHEN** command 包测试覆盖角色创建、更新、启用、停用、重复角色或系统角色保护路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation 表达 `RoleStore` 调用、输入归一化、错误映射和禁止写入路径
- **AND** 系统角色保护相关测试 MUST 明确断言受保护变更不会调用后续写入或 policy change 通知

#### Scenario: 用户角色绑定测试使用生成 mock

- **WHEN** command 包测试覆盖用户角色添加、移除、替换、角色不存在或重复角色 ID 去重路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation、matcher 或 `DoAndReturn` 表达 `RoleStore` 查询、`UserRoleStore` 写入和返回角色集合
- **AND** 用户角色绑定成功后的用户角色缓存失效通知 MUST 通过 `PolicyChangeNotifier` expectation 明确断言

#### Scenario: 角色权限绑定测试使用生成 mock

- **WHEN** command 包测试覆盖角色权限添加、移除、替换、权限不存在、权限不可用或重复权限 ID 去重路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation 表达 `PermissionLookup` 校验和 `RolePermissionStore` 写入
- **AND** 权限查找失败或权限不可用时，测试 MUST 明确断言不会执行角色权限写入或 policy change 通知

#### Scenario: policy change 通知失败被明确覆盖

- **WHEN** 角色写操作、用户角色绑定或角色权限绑定已经成功，但 `PolicyChangeNotifier.NotifyPolicyChanged` 返回错误
- **THEN** 测试 MUST 通过生成 mock expectation 注入通知错误
- **AND** 测试 MUST 断言 command service 返回通知错误，MUST NOT 吞掉通知失败并返回写操作成功结果

#### Scenario: 保留纯测试 helper

- **WHEN** command 包测试需要复用角色、权限引用、输入命令、gomock matcher 或 service fixture 构造逻辑
- **THEN** 保留的 helper MUST NOT 实现 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup` 或 `PolicyChangeNotifier` port
- **AND** 这些 helper MUST NOT 替代生成 mock 记录 collaborator 调用或隐藏失败注入

### Requirement: RBAC seed service 测试协作者契约

RBAC seed service 测试 MUST 使用 `user-service/internal/features/role/application/seed` 包已有 `mockgen` 生成物表达 `SeedRoleStore`、`SeedPermissionStore`、`SeedRolePermissionStore` 和 `SeedUserRoleStore` 等外部持久化协作者契约。测试 MUST NOT 保留实现这些 seed port 的手写 store double 来兼容或隐藏依赖调用、失败路径、调用顺序、重复 seed、reactivate 参数、binding 同步或超级管理员绑定行为。

#### Scenario: 默认 seed 测试使用生成 mock

- **WHEN** seed 包测试覆盖默认系统角色、系统权限和系统角色权限绑定初始化路径
- **THEN** 测试 MUST 通过生成 mock 的 ordered expectation 表达 `SeedRoleStore.UpsertSystemRole`、`SeedPermissionStore.UpsertSystemPermission` 和 `SeedRolePermissionStore.EnsureSystemBindings` 调用
- **AND** 测试 MUST 基于 `rbacbaseline.DefaultRoles()`、`DefaultPermissions()` 和 `DefaultRolePermissions()` 校验调用数量、参数映射和返回统计

#### Scenario: 重复 seed 测试使用生成 mock

- **WHEN** seed 包测试覆盖默认系统数据已经存在的重复 seed 路径
- **THEN** 测试 MUST 通过生成 mock 返回已存在写入结果，并断言 `SeedResult` 的 inserted、updated 和 binding added 统计保持既有语义
- **AND** 测试 MUST NOT 依赖手写 store double 的内部状态模拟重复数据

#### Scenario: reactivate 和 sync bindings 测试使用生成 mock

- **WHEN** seed 包测试覆盖 `ReactivateSystem` 或 `SyncSystemBindings` 选项
- **THEN** 测试 MUST 通过 matcher 明确断言角色和权限 upsert 输入携带正确的 reactivate 参数
- **AND** `SyncSystemBindings` 场景 MUST 通过 `SeedRolePermissionStore.SyncSystemBindings` expectation 表达新增和删除绑定统计

#### Scenario: assign super admin 测试使用生成 mock

- **WHEN** seed 包测试覆盖为用户分配内置超级管理员角色
- **THEN** 测试 MUST 通过 `SeedUserRoleStore.AssignRole` expectation 断言用户 ID 和 `rbacbaseline.SuperAdminRoleID` 对应角色 ID
- **AND** 测试 MUST 覆盖新增绑定和已有绑定两类返回结果

#### Scenario: 保留纯测试 helper

- **WHEN** seed 包测试需要复用 service fixture、输入 matcher、UUID 解析或 baseline 期望构造逻辑
- **THEN** 保留的 helper MUST NOT 实现 `SeedRoleStore`、`SeedPermissionStore`、`SeedRolePermissionStore` 或 `SeedUserRoleStore` port
- **AND** 这些 helper MUST NOT 替代生成 mock 记录 collaborator 调用或隐藏失败注入

### Requirement: common Casbin wrapper 测试断言迁移

`common/security/casbin` 的测试 MUST 使用统一断言规范验证共享 Casbin authorizer wrapper、请求三元组、允许、拒绝、未配置和底层错误路径。断言迁移 MUST 保持 Casbin 三元组授权、`ErrNotConfigured`、`ErrDenied`、返回 bool/error 的 `Enforce` 语义和 error-only `Authorizer.Authorize` 语义不变。

#### Scenario: Casbin 允许和拒绝断言

- **WHEN** `common/security/casbin` 测试验证允许访问、策略拒绝或底层 enforcer 返回错误
- **THEN** 测试 MUST 使用 `require` 表达 bool 结果、错误存在性、错误类型和错误包装断言
- **AND** 迁移 MUST NOT 改变 `Enforce` 或 `Authorizer.Authorize` 的授权结果语义

#### Scenario: 未配置 authorizer 断言

- **WHEN** 测试验证 nil enforcer、未配置 authorizer 或非法请求三元组路径
- **THEN** 测试 MUST 使用语义化断言表达 `ErrNotConfigured`、`ErrDenied` 或参数校验结果
- **AND** 迁移 MUST NOT 将未配置、拒绝访问或底层错误折叠为无法区分的测试结果

#### Scenario: 不影响 user-service RBAC

- **WHEN** common Casbin wrapper 测试迁移断言风格
- **THEN** user-service 的权限目录、角色绑定、用户角色绑定、policy loader、policy sync、超级管理员通配授权和 RBAC HTTP 授权行为 MUST 保持不变
- **AND** 迁移 MUST NOT 修改 user-service feature 测试或 RBAC 生产代码

### Requirement: Permission 测试断言规范

permission feature 的 Go 测试 MUST 优先使用 `testify/require` 表达错误、对象、状态、集合、字符串和授权结果等语义化断言。只有当单个测试需要收集多个互相独立的字段失败，且后续检查不依赖前置检查成功时，测试 MAY 使用 `testify/assert`。permission 测试 MUST NOT 通过机械 `Fail` / `Failf` 替换、自定义兼容 helper、旧字段断言或旧接口断言来保留历史断言形态。

#### Scenario: 迁移 permission 历史断言
- **WHEN** permission catalog、authorization、policy sync、route diff、HTTP boundary、Casbin adapter、PostgreSQL store、Redis watcher 或 metrics 测试检查错误返回、对象字段、布尔状态、集合长度、字符串内容或授权结果
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.NotNil`、`require.True`、`require.False`、`require.Len`、`require.Contains` 等语义化断言表达预期
- **AND** 测试 MUST NOT 使用 `t.Fatalf`、`t.Fatal`、`t.Errorf`、`t.Error` 或 `Fail` 类调用表达已有语义化断言可以清晰覆盖的失败

#### Scenario: 收集互相独立字段失败
- **WHEN** route diff 或多字段 HTTP 响应测试需要在一次执行中展示多个互相独立字段的差异
- **THEN** 测试 MAY 使用 `assert` 收集这些字段失败
- **AND** 初始化失败、错误返回、nil 检查、响应解析或后续检查依赖的前置条件仍然 MUST 使用 `require` 立即终止当前测试

#### Scenario: 保持 collaborator 契约表达
- **WHEN** permission application、transport/http、Casbin adapter、PostgreSQL store 或 Redis watcher 测试依赖已有 gomock 生成物表达外部协作者调用、失败注入或调用顺序
- **THEN** 测试 MUST 保持既有生成 mock 使用方式
- **AND** 本次断言迁移 MUST NOT 回退为手写 collaborator double 或通过 helper 隐藏 collaborator expectation

#### Scenario: 不保留旧兼容断言
- **WHEN** permission 测试迁移断言表达
- **THEN** 测试 MUST NOT 新增旧 permission 字段、旧 route scanner 输出、旧 watcher 签名或旧授权白名单兼容断言
- **AND** 测试 MUST NOT 新增机械 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf` 替换来模拟历史手写失败判断

#### Scenario: 残留手写失败调用符合例外
- **WHEN** `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/permission --glob '*_test.go'` 在迁移后仍有命中
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的场景
- **AND** 实现任务记录 MUST 列明这些剩余命中及其保留原因

### Requirement: Role 与 RBAC baseline 测试断言规范

role feature 和 shared RBAC baseline 的 Go 测试 MUST 优先使用 `testify/require` 表达错误、对象、状态、集合、字符串、HTTP response、store 结果和 baseline catalog 等语义化断言。只有当单个测试需要收集多个互相独立的字段失败，且后续检查不依赖前置检查成功时，测试 MAY 使用 `testify/assert`。role 与 baseline 测试 MUST NOT 通过机械 `Fail` / `Failf` 替换、自定义兼容 helper、旧字段断言、旧 binding 断言或旧 baseline catalog 断言来保留历史断言形态。

#### Scenario: 迁移 role 历史断言

- **WHEN** role command、query、seed、domain、HTTP boundary、PostgreSQL store、RoleStore、UserRoleStore 或 RolePermissionStore 测试检查错误返回、对象字段、布尔状态、集合长度、字符串内容、HTTP status 或绑定结果
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.NotNil`、`require.True`、`require.False`、`require.Len`、`require.Contains` 等语义化断言表达预期
- **AND** 测试 MUST NOT 使用 `t.Fatalf`、`t.Fatal`、`t.Errorf`、`t.Error` 或 `Fail` 类调用表达已有语义化断言可以清晰覆盖的失败

#### Scenario: 迁移 baseline catalog 历史断言

- **WHEN** shared RBAC baseline 测试检查系统角色、系统权限、默认绑定、超级管理员常量或 catalog 唯一性
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 表达错误、相等性、包含关系、空值、非空值、长度、唯一性和布尔预期
- **AND** 测试 MUST NOT 新增旧 baseline 常量、旧 catalog 条目或旧绑定关系兼容断言

#### Scenario: 收集互相独立字段失败

- **WHEN** 多字段 HTTP response、角色列表摘要、权限摘要或 baseline catalog 测试需要在一次执行中展示多个互相独立字段的差异
- **THEN** 测试 MAY 使用 `assert` 收集这些字段失败
- **AND** 初始化失败、错误返回、nil 检查、响应解析、store 连接或后续检查依赖的前置条件仍然 MUST 使用 `require` 立即终止当前测试

#### Scenario: 保持 collaborator 契约表达

- **WHEN** role application、transport/http、seed 或 store 相关测试依赖已有 gomock 生成物表达外部协作者调用、失败注入或调用顺序
- **THEN** 测试 MUST 保持既有生成 mock 使用方式
- **AND** 本次断言迁移 MUST NOT 回退为手写 store double、notifier double、fake 或通过 helper 隐藏 collaborator expectation

#### Scenario: 不保留旧兼容断言

- **WHEN** role 与 baseline 测试迁移断言表达
- **THEN** 测试 MUST NOT 新增旧 role 字段、旧 binding 行为、旧 baseline catalog、旧 fake 或旧 helper 兼容断言
- **AND** 测试 MUST NOT 新增机械 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf` 替换来模拟历史手写失败判断

#### Scenario: 残留手写失败调用符合例外

- **WHEN** `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/role user-service/internal/shared/rbacbaseline --glob '*_test.go'` 在迁移后仍有命中
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的场景
- **AND** 实现任务记录 MUST 列明这些剩余命中及其保留原因

### Requirement: Role HTTP boundary 测试覆盖

role feature 的 HTTP boundary 测试 MUST 直接覆盖角色生命周期、角色权限绑定和用户角色绑定 controller。测试 MUST 固定请求绑定、input preparer、application command/query port 调用、错误映射和 response envelope 的当前契约，并 MUST NOT 通过旧 role 字段、旧请求字段别名、旧 binding 行为、旧 envelope 形态、旧错误码或兼容 helper 表达预期。

#### Scenario: 角色生命周期 handler 成功路径

- **WHEN** role HTTP 测试覆盖角色列表、创建、详情、更新和启停 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用对应 role application command/query port，并传入由当前 URI、query 和 JSON body 归一化得到的 command/query
- **AND** 测试 MUST 验证成功响应使用当前 response envelope、HTTP status 和 role response 字段映射

#### Scenario: 角色权限绑定 handler 成功路径

- **WHEN** role HTTP 测试覆盖查询、替换、新增和移除角色权限绑定 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用对应 role application command/query port，并传入当前 role ID、permission ID 或 permission ID 集合
- **AND** 测试 MUST 验证成功响应使用当前 response envelope、HTTP status 和 permission response 字段映射

#### Scenario: 用户角色绑定 handler 成功路径

- **WHEN** role HTTP 测试覆盖查询、替换、新增和移除用户角色绑定 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用对应 role application command/query port，并传入当前 user ID、role ID 或 role ID 集合
- **AND** 测试 MUST 验证成功响应使用当前 response envelope、HTTP status 和 role response 字段映射

#### Scenario: 请求绑定和输入解析失败

- **WHEN** role HTTP controller 收到非法 URI UUID、非法 cursor、非法 query 参数、非法 JSON body 或缺失必填字段
- **THEN** 测试 MUST 验证请求在 HTTP boundary 被拒绝并返回当前 bad request 或 validation failed envelope
- **AND** 测试 MUST 验证对应 application command/query port 未被调用

#### Scenario: application 错误映射

- **WHEN** role application command/query port 返回 domain、validation、not found、conflict 或内部错误
- **THEN** role HTTP boundary 测试 MUST 验证 controller 通过当前 `toRoleHTTPError` 映射为对应 HTTP status 和 envelope code
- **AND** 测试 MUST NOT 新增旧错误码、旧 message 或旧 envelope 兼容断言

#### Scenario: 保持 role HTTP 测试边界

- **WHEN** role HTTP boundary 测试需要构造 collaborator、请求上下文或响应断言
- **THEN** 测试 MUST 使用现有 gomock 生成物或既有生成入口维护的 mock 表达 application port 调用
- **AND** 测试 MUST NOT 引入 infrastructure store、Ent client、PostgreSQL、Casbin engine、RBAC seed 或跨 feature adapter 作为 controller 单元测试依赖

#### Scenario: 不保留旧兼容路径

- **WHEN** role HTTP boundary 测试新增或调整断言
- **THEN** 测试 MUST NOT 新增旧 role 字段、旧 request body 字段别名、旧 binding 行为、旧 response envelope、旧错误码或旧 helper 兼容断言
- **AND** 测试 MUST 使用 `testify/require` 或必要的 `assert` 表达语义化断言，MUST NOT 使用机械 `Fail` / `Failf` 替换来模拟历史手写失败判断

### Requirement: Role PostgreSQL store 持久化契约

系统 MUST 使用当前 Ent schema 和外部 UUID 字段实现 Role PostgreSQL store 的角色、用户角色绑定和角色权限绑定持久化；join 表内部外键只用于数据库关联，不得暴露为 role feature 的业务查询入口。store MUST 将当前领域错误稳定映射给 application 层，并在替换绑定失败时保持已有绑定不被部分破坏。

#### Scenario: 角色按外部 UUID 持久化和查询

- **WHEN** role infrastructure store 创建、查询、批量查询、列表、更新或启停角色
- **THEN** 系统 MUST 以 `roles.role_id` 作为稳定业务标识执行查询和排序
- **AND** 唯一约束冲突 MUST 映射为 `ErrRoleAlreadyExists`
- **AND** 未找到目标角色 MUST 映射为 `ErrRoleNotFound`

#### Scenario: 用户角色绑定使用当前用户和角色身份

- **WHEN** role infrastructure store 查询、添加、替换或移除用户角色绑定
- **THEN** 系统 MUST 通过用户外部 UUID 和角色外部 UUID 解析当前未软删除用户与角色
- **AND** 空绑定集合 MUST 返回空结果且不得创建兼容占位数据
- **AND** 重复或不存在的绑定引用 MUST 返回当前领域错误并保持已有绑定关系不被破坏

#### Scenario: 角色权限绑定复核当前启用权限

- **WHEN** role infrastructure store 添加或替换角色权限绑定
- **THEN** 系统 MUST 在写入前按权限外部 UUID 复核权限存在且处于启用状态
- **AND** 不存在或已停用权限 MUST 映射为当前权限或角色绑定领域错误
- **AND** 替换失败 MUST 回滚事务并保留替换前的角色权限绑定

#### Scenario: 不引入旧兼容查询和绑定语义

- **WHEN** Role PostgreSQL store 测试或实现覆盖角色与绑定持久化
- **THEN** 系统 MUST NOT 新增旧 internal ID 查询入口、旧 role code 字段、旧 binding 行为或兼容查询 helper
- **AND** 测试 MUST 以当前 Ent schema、当前外部 UUID 字段和当前领域错误为准

### Requirement: Permission HTTP boundary 测试覆盖

permission feature 的 HTTP boundary 测试 MUST 直接覆盖权限目录生命周期、用户有效权限查询和 route diff controller。测试 MUST 固定请求绑定、input preparer、application command/query port 调用、错误直通渲染、分页 envelope、有效权限 response 和 route diff response 的当前契约，并 MUST NOT 通过旧权限资源路径、旧 action/resource 字段语义、旧错误 envelope、旧授权绕过、旧 route scanner 输出或兼容 helper 表达预期。

#### Scenario: 权限目录 handler 成功路径

- **WHEN** permission HTTP 测试覆盖权限列表、创建、详情、更新、启用和停用 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用对应 permission application command/query port，并传入由当前 URI、query 和 JSON body 归一化得到的 command/query
- **AND** 测试 MUST 验证成功响应使用当前 response envelope、HTTP status、分页信息和 permission response 字段映射

#### Scenario: 用户有效权限 handler 成功路径

- **WHEN** permission HTTP 测试覆盖查询用户有效权限 handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用当前 permission query port，并传入当前 user ID
- **AND** 测试 MUST 验证成功响应使用当前 response envelope 和有效权限 response 字段映射

#### Scenario: route diff handler 成功路径

- **WHEN** permission HTTP 测试覆盖 route diff handler 的合法请求
- **THEN** 测试 MUST 验证 controller 调用当前 permission query port 获取 route diff 结果
- **AND** 测试 MUST 验证成功响应使用当前 response envelope 和 missing、stale、mismatch 诊断字段映射

#### Scenario: 请求绑定和输入解析失败

- **WHEN** permission HTTP controller 收到非法 URI UUID、非法 cursor、非法 query 参数、非法 JSON body 或缺失必填字段
- **THEN** 测试 MUST 验证请求在 HTTP boundary 被拒绝并返回当前 bad request 或 validation failed envelope
- **AND** 测试 MUST 验证对应 application command/query port 未被调用

#### Scenario: application 错误直通渲染

- **WHEN** permission application command/query port 返回 domain、validation、not found、conflict 或内部错误
- **THEN** permission HTTP boundary 测试 MUST 验证 controller 通过 `response.Fail(c, err)` 渲染对应 HTTP status 和 envelope code
- **AND** 测试 MUST 覆盖权限已存在、权限不存在、权限输入无效、系统权限保护和未知内部错误响应
- **AND** 测试 MUST NOT 新增旧错误码、旧 message、旧 envelope 或权限专用错误 mapper 兼容断言

#### Scenario: 保持 permission HTTP 测试边界

- **WHEN** permission HTTP boundary 测试需要构造 collaborator、请求上下文或响应断言
- **THEN** 测试 MUST 使用现有 gomock 生成物或既有生成入口维护的 mock 表达 application port 调用
- **AND** 测试 MUST NOT 引入 infrastructure store、Ent client、PostgreSQL、Redis、Casbin engine、RBAC seed 或跨 feature adapter 作为 controller 单元测试依赖

#### Scenario: 语义化断言和不保留旧兼容路径

- **WHEN** permission HTTP boundary 测试新增或调整断言
- **THEN** 测试 MUST 优先使用 `testify/require` 和 `Len`、`Greater`、`ErrorContains`、`ElementsMatch`、`JSONEq`、`Regexp` 等更具体语义化断言
- **AND** 测试 MUST NOT 新增机械 `Fail` / `Failf` / `FailNow` / `FailNowf` 替换、旧权限资源路径、旧 action/resource 字段、旧 binding、旧 response envelope、旧授权绕过或旧 helper 兼容断言

### Requirement: RBAC CLI 测试断言规范

RBAC CLI 测试 MUST 优先使用 `testify/require` 表达 seed、assign-super-admin、create-super-admin、password/env normalization、command construction、error handling 和 cleanup behavior 等语义化断言。只有当单个测试需要收集多个互相独立的命令属性失败，且后续检查不依赖前置检查成功时，测试 MAY 使用 `testify/assert`。

#### Scenario: 迁移 RBAC command 历史断言

- **WHEN** `user-service/cmd` 测试检查 `rbac seed`、`rbac assign-super-admin`、`rbac create-super-admin` 或相关 helper 的错误返回、输出文本、flag/env 归一化、password 输入、cleanup error 或 command metadata
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.Equal`、`require.NotNil`、`require.Len`、`require.Contains`、`require.Regexp` 或等价语义化断言表达预期
- **AND** 测试 MUST NOT 使用 `t.Fatalf`、`t.Fatal`、`t.Errorf`、`t.Error` 或 `Fail` 类调用表达已有语义化断言可以清晰覆盖的失败

#### Scenario: 不保留旧 RBAC CLI 兼容断言

- **WHEN** RBAC CLI 测试迁移断言表达
- **THEN** 测试 MUST NOT 新增旧 root command alias、旧 RBAC command path、旧 flag/env 名、旧 password handling 或旧 cleanup behavior 兼容断言
- **AND** 迁移 MUST NOT 改变 RBAC seed、超级管理员角色绑定、密码哈希、用户状态或权限目录生产语义

#### Scenario: RBAC CLI 残留手写失败调用符合例外

- **WHEN** `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/cmd --glob '*_test.go'` 在迁移后仍有命中
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的场景
- **AND** 实现任务记录 MUST 列明这些剩余命中及其保留原因

### Requirement: RBAC 路由注册测试覆盖
系统 MUST 使用 router 包测试覆盖权限、角色和用户角色路由在 user-service 聚合路由中的注册结果，确保 RBAC 保护接口只注册在当前 `/api/v1` 路由图并经过当前认证和授权中间件链。

#### Scenario: 权限和角色路由注册
- **WHEN** PermissionController 和 RoleController 均已提供给 `registerV1Routes`
- **THEN** 测试 MUST 验证权限目录、route diff、用户有效权限、角色生命周期、角色权限绑定和用户角色绑定路由注册在 `/api/v1` 下
- **AND** 测试 MUST 验证这些路由进入当前认证和 RBAC 授权中间件链

#### Scenario: RBAC 安全依赖缺失拒绝注册
- **WHEN** `RegisterUserServiceHTTPRoutes` 或 `registerV1Routes` 缺少 token version validator、RBAC authorizer、PermissionController 或 RoleController 任一安全依赖
- **THEN** user-service 聚合路由注册 MUST 返回明确错误
- **AND** 系统 MUST NOT 注册部分 `/api/v1` 业务路由
- **AND** 测试 MUST NOT 通过可选 controller 条件跳过、旧路径兼容别名或部分路由注册补偿缺失依赖

### Requirement: RBAC CLI 命令测试覆盖

`rbac-access-control` 的 user-service RBAC CLI 测试 MUST 直接覆盖当前 `user-service rbac` seed、assign-super-admin 和 create-super-admin 命令契约。测试 MUST 固定当前配置来源、参数归一化、依赖装配、错误传播和 cleanup 语义，并 MUST NOT 为旧命令名、旧 flag、旧环境变量、旧 root Makefile 无服务前缀入口或旧 bootstrap 行为新增兼容断言。

#### Scenario: seed 命令 runner 传递当前选项

- **WHEN** `runRBACSeedCommand` 使用当前配置路径和 seed options 执行
- **THEN** 测试 MUST 验证 runner 通过当前 RBAC seed service 接收 `reactivateSystem` 和 `syncSystemBindings`
- **AND** 测试 MUST 验证成功路径执行 cleanup

#### Scenario: assign-super-admin 命令 runner 绑定指定用户

- **WHEN** `runAssignSuperAdminCommand` 收到合法用户 UUID
- **THEN** 测试 MUST 验证 runner 将该 UUID 传递给当前超级管理员绑定流程
- **AND** 测试 MUST 覆盖绑定已存在和新增绑定两类当前输出语义

#### Scenario: create-super-admin 命令 runner 使用当前创建流程

- **WHEN** `runCreateSuperAdminCommand` 收到当前 create-super-admin options
- **THEN** 测试 MUST 验证 runner 使用当前配置路径初始化依赖并调用 `createSuperAdmin`
- **AND** 测试 MUST 验证输出中的 username 使用当前 username 归一化规则

#### Scenario: createSuperAdmin 覆盖用户存在性分支

- **WHEN** `createSuperAdmin` 处理不存在用户、已存在用户不重置密码或已存在用户重置密码
- **THEN** 测试 MUST 验证新建用户、角色绑定、密码 hash 和凭据更新按当前契约发生
- **AND** 测试 MUST 验证用户读取、创建、hash、凭据更新和角色绑定错误会 fail-fast 返回

#### Scenario: RBAC CLI 初始化和 cleanup 错误可见

- **WHEN** RBAC CLI 依赖初始化失败或命令执行后 cleanup 返回错误
- **THEN** 测试 MUST 验证命令返回明确错误
- **AND** 如果命令错误和 cleanup 错误同时存在，测试 MUST 验证两者都保留在返回错误中

### Requirement: RBAC CLI 参数归一化测试

`rbac-access-control` 的 create-super-admin 参数归一化测试 MUST 固定当前 username、nickname、password env、password value 和 reset password 语义。测试 MUST 只验证当前 `ADMIN_PASSWORD` 默认来源和当前 flag/env 契约，不得新增旧环境变量或旧默认值兼容路径。

#### Scenario: create-super-admin 使用默认 password env

- **WHEN** `passwordEnv` 为空且 `ADMIN_PASSWORD` 存在
- **THEN** `normalizeCreateSuperAdminOptions` MUST 使用 `ADMIN_PASSWORD`
- **AND** username MUST trim 后转小写，空 nickname MUST 回退为归一化 username

#### Scenario: create-super-admin 拒绝缺失必要输入

- **WHEN** password env 不存在、username 为空或 password value 为空
- **THEN** `normalizeCreateSuperAdminOptions` MUST 返回明确错误
- **AND** 测试 MUST 使用 `require.ErrorContains` 或等价语义化断言表达错误内容

#### Scenario: reset password 标志保持当前值

- **WHEN** create-super-admin options 启用 reset password
- **THEN** 归一化结果 MUST 保留 `resetPassword=true`
- **AND** 测试 MUST NOT 通过旧 flag 或旧环境变量表达重置密码预期

### Requirement: 受保护 HTTP flow 授权边界断言规范
系统 MUST 使用语义化断言覆盖 user-service E2E HTTP flow 中受保护用户接口的认证和授权边界。断言迁移 MUST 保持当前认证中间件、RBAC 授权中间件、错误 envelope 和受保护路由语义不变。

#### Scenario: 授权上下文访问用户接口
- **WHEN** E2E flow 使用当前测试前置条件中的有效 bearer token 访问用户创建或用户详情接口
- **THEN** 测试 MUST 使用语义化断言验证请求进入当前受保护 HTTP flow 并返回预期 response envelope
- **AND** 迁移 MUST NOT 绕过认证或 RBAC 中间件，也 MUST NOT 新增旧授权兼容断言

#### Scenario: 缺失认证访问受保护接口
- **WHEN** E2E flow 未提供 bearer token 访问受保护用户接口
- **THEN** 测试 MUST 使用语义化断言验证 HTTP `401 Unauthorized`、`success=false` 和 `CodeUnauthenticated`
- **AND** 测试 MUST NOT 接受旧认证绕过路径、旧错误码或旧 envelope 格式

#### Scenario: 跨 feature 响应断言保持边界
- **WHEN** E2E flow 同时经过认证、RBAC 和用户资料 feature 的响应边界
- **THEN** 测试 MUST 只迁移断言表达
- **AND** 测试 MUST NOT 修改 Casbin policy、RBAC seed、角色权限绑定、用户角色绑定、受保护路由路径或授权结果语义

### Requirement: 角色权限绑定基础设施关键路径测试

role infrastructure MUST 提供默认可执行的测试覆盖角色权限绑定中不依赖 PostgreSQL 行锁的持久化路径，包括列表、删除、系统绑定同步、缺失权限、重复输入去重、失败保持和映射 helper。依赖 `FOR UPDATE` 的新增和替换路径 MUST 保持生产 PostgreSQL 锁语义不变，并 MAY 由显式 Docker-backed PostgreSQL 集成测试覆盖。

#### Scenario: 默认测试覆盖非锁定绑定路径
- **WHEN** 协作者执行 `go test -cover ./user-service/internal/features/role/infrastructure/postgres`
- **THEN** 测试 MUST 默认执行 `RolePermissionStore.ListByRoleID`、`Remove`、`EnsureSystemBindings`、`SyncSystemBindings` 和映射 helper 的成功与错误路径
- **AND** 默认覆盖率 MUST 达到 70% 以上

#### Scenario: 同步失败保持原绑定
- **WHEN** `RolePermissionStore.SyncSystemBindings` 请求引用缺失权限
- **THEN** 测试 MUST 断言方法返回明确错误
- **AND** 测试 MUST 断言失败前已有角色权限绑定保持不变

#### Scenario: 默认测试覆盖系统绑定同步
- **WHEN** 默认测试执行 `RolePermissionStore.SyncSystemBindings`
- **THEN** 测试 MUST 覆盖新增缺失绑定、删除多余绑定和保留既有绑定
- **AND** 测试 MUST 断言返回的新增与删除统计符合持久化结果

#### Scenario: 同步失败保持可诊断结果
- **WHEN** `SyncSystemBindings` 因缺失权限、查询失败或事务写入失败无法完成
- **THEN** 测试 MUST 覆盖错误映射或 rollback 路径
- **AND** 测试 MUST 断言不会把部分成功伪装为完整同步成功

#### Scenario: PostgreSQL 集成测试不承担默认覆盖唯一来源
- **WHEN** `AEGISCORE_TEST_CONTAINERS` 未设置
- **THEN** 默认测试 MAY 跳过 Docker-backed PostgreSQL 集成测试
- **AND** 默认测试仍 MUST 覆盖角色权限绑定非锁定核心路径
- **AND** 生产代码 MUST NOT 为 SQLite 测试新增跳过 `FOR UPDATE` 的兼容分支

### Requirement: RBAC Enforce 低基数指标

系统 MUST 为每次 permission authorization service 的 RBAC Enforce 判定导出低基数 Prometheus metrics，用于观察授权通过、授权拒绝、授权异常和授权耗时。指标 MUST 不改变 Casbin policy、用户角色缓存、policy sync、超级管理员通配授权或 HTTP 授权结果语义。

#### Scenario: 授权通过指标

- **WHEN** 已认证用户拥有当前 HTTP method 和 route template 对应权限，RBAC Enforce 返回允许
- **THEN** 系统 MUST 将 RBAC Enforce counter 记录为 `result="allow"`
- **AND** 系统 MUST 将本次 Enforce 耗时写入 RBAC Enforce latency histogram
- **AND** 指标标签 MUST 只包含 `result`、`method` 和 `route_template`

#### Scenario: 授权拒绝指标

- **WHEN** 已认证用户缺少当前 HTTP method 和 route template 对应权限，RBAC Enforce 返回拒绝
- **THEN** 系统 MUST 将 RBAC Enforce counter 记录为 `result="deny"`
- **AND** 系统 MUST 将本次 Enforce 耗时写入 RBAC Enforce latency histogram
- **AND** 系统 MUST 保持授权拒绝响应语义不变

#### Scenario: 授权异常指标

- **WHEN** RBAC Enforce 因非法 subject、context 取消、用户角色回源失败或 Casbin 执行失败返回错误
- **THEN** 系统 MUST 将 RBAC Enforce counter 记录为 `result="error"`
- **AND** 系统 MUST 将本次 Enforce 耗时写入 RBAC Enforce latency histogram
- **AND** 系统 MUST 保持 fail-closed 行为，不得因指标记录失败放行请求

#### Scenario: Enforce 指标禁止高基数字段

- **WHEN** 系统记录 RBAC Enforce metrics
- **THEN** 指标 MUST NOT 包含用户 ID、角色 ID、权限 ID、token ID、trace/span ID、raw path、IP、邮箱、用户名、Redis key、SQL、SQL 参数或原始错误
- **AND** route 标签 MUST 使用 Gin route template 或等价稳定模板，不得使用真实请求 path

### Requirement: 权限目录错误应用错误渲染

系统 MUST 将权限目录能力中的权限已存在、权限不存在、权限输入无效和系统权限保护错误表达为可由共享 response helper 直接渲染的应用错误，并保持权限 HTTP 边界无专用 sentinel-to-HTTP 兼容映射。

#### Scenario: 权限已存在渲染为冲突响应

- **WHEN** 权限创建或更新流程返回 `permissiondomain.ErrPermissionAlreadyExists`
- **THEN** 权限 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前权限已存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `permission_already_exists`

#### Scenario: 权限不存在渲染为未找到响应

- **WHEN** 权限详情查询、更新或启停流程返回 `permissiondomain.ErrPermissionNotFound`
- **THEN** 权限 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前权限不存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `permission_not_found`

#### Scenario: 权限输入无效渲染为 validation 响应

- **WHEN** 权限 domain validation 返回 `permissiondomain.ErrPermissionInvalid`
- **THEN** 权限 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `400 Bad Request` 和共享 validation 业务 code
- **AND** 响应 message MUST 使用当前权限输入无效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `permission_invalid`

#### Scenario: 系统权限保护渲染为冲突响应

- **WHEN** 权限更新流程返回 `permissiondomain.ErrSystemPermissionProtected`
- **THEN** 权限 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前系统权限保护中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `system_permission_protected`

#### Scenario: 权限业务错误保留 errors.Is 语义

- **WHEN** permission feature 或测试需要判断权限已存在、权限不存在、权限输入无效或系统权限保护错误
- **THEN** `errors.Is` 对直接返回的权限应用错误和被包装后的权限应用错误 MUST 继续支持正确匹配
- **AND** 系统 MUST NOT 为 permission HTTP transport 保留 `toPermissionHTTPError` 或等价兼容函数

### Requirement: 权限 HTTP transport 统一错误出口

permission HTTP transport MUST 对业务 command/query 返回错误使用共享 `response.Fail` 入口，避免在 transport 层重复维护权限目录错误到 HTTP 响应的映射。授权中间件错误处理 MUST 继续使用共享 response helper，且不得复用或新增权限目录错误 mapper。

#### Scenario: 权限创建业务错误

- **WHEN** `CreatePermission` controller 调用权限创建 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限详情查询业务错误

- **WHEN** `GetPermission` controller 调用权限查询 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限列表查询业务错误

- **WHEN** `ListPermissions` controller 调用权限列表 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限更新业务错误

- **WHEN** `UpdatePermission` controller 调用权限更新 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限启停业务错误

- **WHEN** `EnablePermission` 或 `DisablePermission` controller 调用权限启停 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 权限有效权限与 route diff 业务错误

- **WHEN** `ListEffectivePermissions` 或 `DiffRoutes` controller 调用权限 query use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用权限专用错误 mapper

#### Scenario: 授权 HTTP transport 不使用权限目录 mapper

- **WHEN** permission HTTP 授权中间件处理缺失认证、策略拒绝或授权执行错误
- **THEN** 授权中间件 MUST 使用共享 response helper 渲染当前未认证、禁止访问或内部错误响应
- **AND** 授权中间件 MUST NOT 调用 `toPermissionHTTPError` 或任何权限目录错误 mapper

### Requirement: 角色与绑定错误应用错误渲染

系统 MUST 将角色目录、用户角色绑定和角色权限绑定能力中的稳定业务错误表达为可由共享 response helper 直接渲染的应用错误，并保持 role HTTP 边界无专用 sentinel-to-HTTP 兼容映射。

#### Scenario: 角色已存在渲染为冲突响应

- **WHEN** 角色创建或更新流程返回 `roledomain.ErrRoleAlreadyExists`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前角色已存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_already_exists`

#### Scenario: 角色不存在渲染为未找到响应

- **WHEN** 角色详情查询、更新、启停、用户角色绑定或角色权限绑定流程返回 `roledomain.ErrRoleNotFound`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前角色不存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_not_found`

#### Scenario: 角色输入无效渲染为 validation 响应

- **WHEN** 角色 domain validation 返回 `roledomain.ErrRoleInvalid`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `400 Bad Request` 和共享 validation 业务 code
- **AND** 响应 message MUST 使用当前角色输入无效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_invalid`

#### Scenario: 系统角色保护渲染为冲突响应

- **WHEN** 角色更新或启停流程返回 `roledomain.ErrSystemRoleProtected`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前系统角色保护中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `system_role_protected`

#### Scenario: 停用角色渲染为冲突响应

- **WHEN** 用户角色绑定流程返回 `roledomain.ErrRoleInactive`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前角色已停用中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_inactive`

#### Scenario: 用户角色绑定已存在渲染为冲突响应

- **WHEN** 用户角色增量绑定流程返回 `roledomain.ErrUserRoleAlreadyExists`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前用户角色绑定已存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `user_role_already_exists`

#### Scenario: 用户角色绑定不存在渲染为未找到响应

- **WHEN** 用户角色解绑流程返回 `roledomain.ErrUserRoleNotFound`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前用户角色绑定不存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `user_role_not_found`

#### Scenario: 角色权限绑定已存在渲染为冲突响应

- **WHEN** 角色权限增量绑定流程返回 `roledomain.ErrRolePermissionAlreadyExists`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前角色权限绑定已存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_permission_already_exists`

#### Scenario: 角色权限绑定不存在渲染为未找到响应

- **WHEN** 角色权限解绑或绑定查询流程返回 `roledomain.ErrRolePermissionNotFound`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前角色权限绑定不存在中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `role_permission_not_found`

#### Scenario: 跨 feature 不存在错误透传

- **WHEN** 用户角色绑定流程返回 `identity.ErrUserNotFound` 或角色权限绑定流程返回 `permissiondomain.ErrPermissionNotFound`
- **THEN** role HTTP 边界 MUST 通过 `response.Fail(c, err)` 直接透传渲染失败响应
- **AND** 用户不存在 MUST 返回 `404 Not Found` 和用户身份错误自身携带的共享未找到业务 code 与公开 message
- **AND** 权限不存在 MUST 返回 `404 Not Found` 和权限目录错误自身携带的共享未找到业务 code 与公开 message
- **AND** role HTTP transport MUST NOT 复制 identity 或 permission 错误映射

#### Scenario: 角色业务错误保留 errors.Is 语义

- **WHEN** role feature、seed 或测试需要判断角色目录、用户角色绑定或角色权限绑定错误
- **THEN** `errors.Is` 对直接返回的角色应用错误和被包装后的角色应用错误 MUST 继续支持正确匹配
- **AND** 系统 MUST NOT 为 role HTTP transport 保留 `toRoleHTTPError` 或等价兼容函数

### Requirement: 角色 HTTP transport 统一错误出口

role HTTP transport MUST 对业务 command/query 返回错误使用共享 `response.Fail` 入口，避免在 transport 层重复维护角色、用户角色绑定、角色权限绑定、identity 或 permission 错误到 HTTP 响应的映射。

#### Scenario: 角色目录 controller 业务错误

- **WHEN** `ListRoles`、`CreateRole`、`GetRole`、`UpdateRole` 或 `SetRoleStatus` controller 调用角色 command/query use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用角色专用错误 mapper

#### Scenario: 用户角色 controller 业务错误

- **WHEN** `ListUserRoles`、`ReplaceUserRoles`、`AddUserRole` 或 `RemoveUserRole` controller 调用用户角色 command/query use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用角色专用错误 mapper

#### Scenario: 角色权限 controller 业务错误

- **WHEN** `ListRolePermissions`、`ReplaceRolePermissions`、`AddRolePermission` 或 `RemoveRolePermission` controller 调用角色权限 command/query use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用角色专用错误 mapper

#### Scenario: role transport 不保留跨 feature 错误 mapper

- **WHEN** role HTTP transport 接收来自 identity、permission 或 role domain 的应用错误
- **THEN** controller MUST 通过共享 response helper 渲染错误自身携带的 HTTP status、code 和 message
- **AND** role HTTP transport MUST NOT 新增或保留将 role、identity 或 permission sentinel error 转换为 HTTP 应用错误的 mapper

### Requirement: 角色 HTTP boundary 测试覆盖应用错误直通

role feature 的 HTTP boundary 测试 MUST 覆盖角色目录、用户角色绑定和角色权限绑定 controller 的应用错误直通渲染。测试 MUST 固定请求绑定、input preparer、application command/query port 调用、错误直通渲染、角色 response 和权限 response 的当前契约，并 MUST NOT 通过旧错误 mapper 表达预期。

#### Scenario: 角色目录错误直通渲染

- **WHEN** role HTTP 测试覆盖角色创建、详情、更新或启停 handler 的业务错误
- **THEN** 测试 MUST 验证 controller 通过 `response.Fail(c, err)` 渲染角色已存在、角色不存在、角色输入无效、系统角色保护和未知内部错误响应
- **AND** 测试 MUST NOT 依赖 `toRoleHTTPError` 或等价兼容函数

#### Scenario: 用户角色错误直通渲染

- **WHEN** role HTTP 测试覆盖用户角色替换、增量绑定或解绑 handler 的业务错误
- **THEN** 测试 MUST 验证 controller 通过 `response.Fail(c, err)` 渲染角色不存在、角色停用、用户角色绑定已存在、用户角色绑定不存在、用户不存在和未知内部错误响应
- **AND** 测试 MUST NOT 在 role transport 层复制 identity 错误映射

#### Scenario: 角色权限错误直通渲染

- **WHEN** role HTTP 测试覆盖角色权限替换、增量绑定或解绑 handler 的业务错误
- **THEN** 测试 MUST 验证 controller 通过 `response.Fail(c, err)` 渲染角色不存在、权限不存在、角色权限绑定已存在、角色权限绑定不存在和未知内部错误响应
- **AND** 测试 MUST NOT 在 role transport 层复制 permission 错误映射

#### Scenario: 保持 role HTTP 测试边界

- **WHEN** role HTTP boundary 测试需要构造 collaborator、请求上下文或响应断言
- **THEN** 测试 MUST 使用现有 gomock 生成物或既有生成入口维护的 mock 表达 application port 调用
- **AND** 测试 MUST NOT 引入 infrastructure store、Ent client、PostgreSQL、Redis、Casbin engine、RBAC seed 或跨 feature adapter 作为 controller 单元测试依赖
