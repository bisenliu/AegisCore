## Purpose

定义 user-service 的 RBAC 访问控制能力，覆盖权限目录、角色、角色权限、用户角色、Casbin 授权、系统 seed 和超级管理员引导。

## Requirements

### Requirement: 权限目录管理

系统 MUST 提供权限目录创建、更新、启停、查询、列表和路由差异分析能力，用于描述可授权的 HTTP 资源和动作。

#### Scenario: 创建权限

- **WHEN** 授权调用方提供合法权限标识、方法、路径和描述
- **THEN** 系统 MUST 创建权限记录，并使其可参与后续角色绑定和授权判断

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

系统 MUST 提供角色创建、更新、查询、列表和角色权限绑定能力，并保证绑定引用的权限存在且状态可用。

#### Scenario: 创建角色并绑定权限

- **WHEN** 授权调用方创建角色并指定合法权限集合
- **THEN** 系统 MUST 持久化角色、写入角色权限绑定，并使授权策略可同步使用

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

### Requirement: 用户角色绑定

系统 MUST 支持将角色绑定到用户，并为授权判断提供用户有效权限查询能力。

#### Scenario: 绑定角色给用户

- **WHEN** 授权调用方把存在的角色绑定给存在用户
- **THEN** 系统 MUST 写入用户角色关系，并使该用户后续访问权限生效

#### Scenario: 用户或角色不存在

- **WHEN** 用户角色绑定请求引用不存在的用户或角色
- **THEN** 系统 MUST 拒绝绑定并返回明确错误

#### Scenario: 查询用户有效权限

- **WHEN** 系统或调用方查询某用户有效权限
- **THEN** 系统 MUST 聚合用户角色和角色权限，返回该用户当前可访问的权限集合

#### Scenario: 用户无角色

- **WHEN** 已认证用户没有有效角色绑定并访问 RBAC 保护路由
- **THEN** 系统 MUST 拒绝访问

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

系统 MUST 使用 RBAC 授权中间件保护权限、角色和用户业务接口，并在认证通过后执行资源级授权判断。Casbin subject/object/action MUST 分别使用 `user:<user_uuid>`、`role:<role_uuid>`、Gin route template 和 HTTP method。

#### Scenario: 授权通过

- **WHEN** 已认证用户拥有当前 HTTP 方法和路径对应权限
- **THEN** 系统 MUST 允许请求进入目标 controller

#### Scenario: 授权失败

- **WHEN** 已认证用户缺少当前 HTTP 方法和路径对应权限
- **THEN** 系统 MUST 拒绝请求并返回授权失败错误

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
- **THEN** policy loader MUST 补充 wildcard policy，使其可访问受保护业务接口，且 MUST NOT 在 role 或 permission feature 内重复定义超级管理员常量

### Requirement: 策略同步

系统 MUST 在在线 RBAC 写操作成功后触发本实例策略刷新，并通过 Redis policy version、Pub/Sub 和定时版本补偿同步其他副本。

#### Scenario: 在线角色绑定变更

- **WHEN** 用户角色绑定通过 HTTP API 变更成功
- **THEN** 本实例 MUST 执行策略刷新或用户角色缓存失效，并通知其他副本

#### Scenario: 在线权限策略变更

- **WHEN** 在线 RBAC 管理接口修改权限、角色启停或角色权限绑定并提交成功
- **THEN** 本实例 MUST 执行 policy reload，并通过 Redis policy version 和 Pub/Sub 通知其他副本；其他副本 MUST 通过 Pub/Sub 和周期性版本补偿感知变更

#### Scenario: 授权热路径

- **WHEN** 业务请求进入 RBAC 授权中间件
- **THEN** 授权 MUST 使用本实例内存 Casbin enforcer 和本地可用的用户角色解析结果，MUST NOT 每请求读取 Redis policy version 做强一致门控

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
