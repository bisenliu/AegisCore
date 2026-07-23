## Purpose

定义 user-service 的 RBAC 访问控制能力，覆盖权限目录、角色、角色权限、用户角色、Casbin 授权、策略同步、系统数据引导、超级管理员管理、错误契约、观测和资源生命周期。
## Requirements
### Requirement: 权限目录与路由诊断

系统 MUST 将 `internal/shared/rbacbaseline.DefaultPermissions()` 作为权限定义的唯一业务权威来源，并将 permissions 数据库表作为供列表、角色绑定和授权加载使用的只读投影。权限 MUST 使用稳定 `permission_id` 描述可授权的 HTTP method、route template 和业务模块；运行时 MUST NOT 提供权限创建、详情、更新、启停或 route diff 公开接口。

#### Scenario: 权限查询边界

- **WHEN** user-service 注册权限 HTTP 路由
- **THEN** 系统 MUST 只注册 `GET /api/v1/permissions` 和 `GET /api/v1/permissions/users/:user_id/effective`
- **AND** 系统 MUST NOT 注册权限创建、详情、更新、启用、停用或 route diff HTTP 路由
- **WHEN** 授权调用方查询权限目录
- **THEN** 系统 MUST 按稳定权限 ID 排序返回完整匹配权限投影集合
- **AND** 权限列表请求 MUST 只支持 `module` 和 `http_method` 过滤参数，MUST NOT 接受或展示 `cursor` 或 `page_size` 分页参数
- **AND** 权限列表成功响应 MUST 使用 `data.items` 包装权限集合，MUST NOT 包含 `data.pagination`
- **AND** 列表输入和响应 MUST NOT 包含 `active`、`is_system` 或 `system`
- **WHEN** 授权调用方使用非法 `http_method` 查询权限目录
- **THEN** 系统 MUST 返回 `400 Bad Request`

#### Scenario: 权限定义和 seed 投影

- **WHEN** 运维执行 RBAC seed
- **THEN** 系统 MUST 按 `rbacbaseline.DefaultPermissions()` 中的稳定 `permission_id` 幂等 upsert 权限名称、描述、模块、HTTP method 和 route template
- **AND** method 或 route template 修改 MUST 沿用原 `permission_id`，使已有角色权限绑定保持不变
- **AND** 权限实体、seed 输入和数据库投影 MUST NOT 包含权限启停或系统权限标记

#### Scenario: 受控删除权限

- **WHEN** 权限从 `rbacbaseline.DefaultPermissions()` 删除
- **THEN** 受控 migration MUST 先删除对应 `role_permissions` 再删除 `permissions` 记录
- **AND** seed 和 HTTP 运行时 MUST NOT 自动删除基线之外的权限或绑定
- **AND** 数据清理后系统 MUST 通过显式 policy reload 或滚动重启使 Casbin policy 收敛

#### Scenario: 路由与权限基线一致性门禁

- **WHEN** CI 或测试构建真实 Gin route graph 并扫描 `/api/v1` 下需要 RBAC 授权的路由
- **THEN** 系统 MUST 将 HTTP method 和 route template 与 `rbacbaseline.DefaultPermissions()` 双向比较
- **AND** 任一实际路由缺少基线权限或任一基线权限没有对应实际路由时测试 MUST 失败
- **AND** 扫描 MUST 排除 `OPTIONS`、认证公开接口和会话控制接口
- **AND** 一致性校验 MUST NOT 创建或修改权限、角色绑定或运行时 policy

### Requirement: 角色目录与角色权限绑定

系统 MUST 提供角色创建、更新、启停、详情、列表和角色权限绑定能力。公开写接口 MUST NOT 允许调用方创建或篡改系统角色；角色权限关系 MUST 只引用存在的代码基线权限，并在完整替换时保持事务性。

#### Scenario: 角色目录写入和查询

- **WHEN** 授权调用方提交合法角色信息和存在的权限集合
- **THEN** 系统 MUST 创建非系统角色并写入角色权限绑定，成功响应 MUST 返回新建角色实体
- **WHEN** 授权调用方使用合法输入更新、启用或停用存在的普通角色
- **THEN** 系统 MUST 持久化对应变更，成功响应 MUST NOT 包含更新后的角色实体
- **WHEN** 授权调用方查询角色详情或分页查询角色
- **THEN** 系统 MUST 返回角色数据、权限摘要和共享 pagination 信息

#### Scenario: 系统角色和权限绑定保护

- **WHEN** 普通角色接口尝试创建、修改或停用系统角色
- **THEN** 系统 MUST 拒绝操作并保持系统角色及其基线语义不变
- **WHEN** 角色权限写请求引用不存在或不属于当前代码基线投影的权限
- **THEN** 系统 MUST 拒绝写入并保持已有角色权限关系不变
- **AND** role application MUST 通过 permission application 拥有的最小查询端口校验权限，MUST NOT 导入 permission infrastructure

#### Scenario: 普通角色绑定基线权限

- **WHEN** 授权调用方把 `rbacbaseline.DefaultPermissions()` 中任意存在的权限绑定给普通角色
- **THEN** 系统 MUST 允许绑定且 MUST NOT 执行权限 active 或 system 状态检查
- **AND** Permission 的状态语义移除 MUST NOT 删除或改变 `Role.Active` 与 `Role.IsSystem`

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

系统 MUST 支持查询、添加、移除和完整替换用户角色绑定，并基于用户当前启用角色及其权限绑定提供有效权限。写路径 MUST 校验用户和角色状态，失败时 MUST 保持原绑定和同步状态不变。

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
- **THEN** 系统 MUST 聚合该用户当前启用角色及其绑定的存在权限并返回去重后的权限集合
- **AND** 有效权限响应 MUST NOT 包含权限 `active`、`is_system` 或 `system`
- **AND** 角色、权限、用户角色和角色权限 MUST 使用外部 UUID 作为稳定业务标识，join 表内部标识 MUST NOT 暴露给 application 或 transport
- **WHEN** 已认证用户没有有效角色绑定并访问受 RBAC 保护的路由
- **THEN** 系统 MUST 拒绝访问

### Requirement: Casbin 授权与 HTTP 保护

系统 MUST 在认证通过后使用 RBAC 授权中间件保护权限、角色和用户业务接口。授权 MUST 使用用户与角色的稳定 subject、Gin route template object 和 HTTP method action，并在任何身份、策略或执行异常下 fail-closed。

#### Scenario: 构造和执行授权请求

- **WHEN** 已认证请求进入受 RBAC 保护的 `/api/v1` 路由
- **THEN** 中间件 MUST 使用请求上下文中的用户 ID、Gin `FullPath()` 和 HTTP method 构造授权请求
- **AND** 用户 subject MUST 使用 `user:<user_uuid>`，角色 subject MUST 使用 `role:<role_uuid>`
- **WHEN** 用户当前启用角色拥有匹配 HTTP method 和 route template 的权限绑定
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
- **THEN** policy MUST 从启用角色、角色权限绑定、permissions 投影和用户角色绑定派生
- **AND** policy loader MUST NOT 使用权限 active predicate，独立 `casbin_rules` 表 MUST NOT 成为业务权威来源，用户身份解析 MUST 排除已软删除用户
- **WHEN** 用户拥有 `internal/shared/rbacbaseline` 定义的内置超级管理员角色
- **THEN** policy loader MUST 为该角色提供受保护业务接口的 wildcard policy
- **AND** 超级管理员角色常量 MUST 只由 `rbacbaseline` 提供

### Requirement: RBAC 策略加载、缓存与多副本同步

系统 MUST 以 PostgreSQL 关系数据作为业务权威投影，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。系统 MUST 在启动时显式加载 policy，在线角色状态、角色权限绑定和用户角色绑定写操作成功后刷新本实例状态，并通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本。授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

#### Scenario: 初始加载和恢复

- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 显式调用初始 policy 加载入口，并将可取消或带超时的启动 context 传给 policy loader
- **WHEN** 初始 policy 加载失败或被取消
- **THEN** engine MUST 记录最近错误和 reload 失败指标，后续授权 MUST fail-closed，`app.Start` MUST 保持成功
- **AND** reload 状态和 readiness/startup 可观测性 MUST 保留失败信息并拒绝接入业务流量
- **WHEN** 后续 Pub/Sub、版本补偿或显式 reload 成功加载当前 policy
- **THEN** engine MUST 替换为最新可用 policy、清除最近 reload 错误并恢复 readiness/startup

#### Scenario: 用户角色缓存 key 与容量

- **WHEN** RBAC user-role cache 被启用
- **THEN** permission feature MUST 使用 `uuid.UUID` 作为真实业务 key，并将配置的正数 size 映射为最大 item 数
- **AND** common MUST NOT 字符串化 UUID、接收 key encoder 或暴露底层 cache option

#### Scenario: 用户角色缓存命中与 value 隔离

- **WHEN** 用户角色缓存命中
- **THEN** 授权 MUST 使用缓存中角色 ID 的防御性副本，调用方对返回 slice 的修改 MUST NOT 污染缓存内部值或后续读取
- **AND** permission feature MUST 在 loader 写入缓存前及 `RolesForUser` 返回调用方前复制 `[]uuid.UUID`
- **AND** `common/runtime/localcache` MUST NOT 承担角色 ID clone 语义

#### Scenario: 用户角色缓存未命中与关闭

- **WHEN** 用户角色缓存未命中
- **THEN** 系统 MUST 合并同一 `uuid.UUID` 用户的并发回源并查询 PostgreSQL 中的当前启用角色，loader 错误 MUST NOT 写入缓存
- **WHEN** user-role cache 已关闭或回源失败
- **THEN** 授权 MUST fail-closed，MUST NOT 因 cache 不可用产生允许结果
- **WHEN** `rbac.user_role_cache.enabled` 为 false
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice 并保持正确的 fail-closed 授权语义
- **AND** direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次回源结果

#### Scenario: 在线写后同步

- **WHEN** 角色状态、角色权限绑定或用户角色绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST reload policy 或失效相关用户缓存，并发布新的 Redis policy version 和 Pub/Sub 通知
- **AND** 持久化成功后的 reload、缓存失效、version 发布或通知失败 MUST 向调用方返回同步错误，成功响应 MUST NOT 掩盖该错误
- **AND** `PolicyChangeNotifier` MUST 是对应正式 command service 的必需依赖
- **WHEN** 权限投影由离线 migration 或 RBAC seed 改变
- **THEN** 离线命令 MUST NOT 宣称已完成在线 policy refresh，运维 MUST 显式 reload 或滚动重启副本

#### Scenario: reload、发布和副本收敛

- **WHEN** policy refresh coordinator 执行本地 reload 和 version 发布
- **THEN** 本地 reload 失败后系统 MUST 仍尝试发布 version
- **AND** 两者同时失败时返回错误 MUST 保留两项失败，只有两者均成功时系统才 MUST 标记本实例已应用该 version
- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现远端 policy version 更新
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛

### Requirement: RBAC 系统数据与运维 CLI

系统 MUST 提供带服务上下文的 `rbac seed` 和一次性 `rbac bootstrap-super-admin` 命令，用于维护系统角色、代码定义权限投影、默认绑定和全新数据库的首次超级管理员引导。系统角色、系统权限、默认绑定和 bootstrap 用户 ID MUST 由 `internal/shared/rbacbaseline` 以手写保留 UUID 常量定义，全部权限定义 MUST 只来自 `rbacbaseline.DefaultPermissions()`。RBAC 运维 CLI MUST 通过 `aegiscore-user-service` 根命令调用，旧 `aegiscore-user-services` 根命令 MUST NOT 作为 RBAC 兼容入口保留。

#### Scenario: 初始化系统基线

- **WHEN** 运维执行 `aegiscore-user-service rbac seed`
- **THEN** 系统 MUST 幂等创建或更新基线角色、权限投影和绑定并输出变更统计
- **AND** 系统角色 MUST 保持系统数据标记，Permission MUST NOT 包含 `Active` 或 `IsSystem`
- **AND** seed MUST NOT 创建业务用户、自动分配超级管理员或自动删除基线之外的权限记录
- **AND** 非 seed 的公开 HTTP 路径 MUST NOT 创建、修改或启停权限
- **AND** seed 写入的系统角色和权限 ID MUST 引用 `rbacbaseline` 中的固化常量，MUST NOT 在 seed 运行时调用 `id.NewUUID()`、UUIDv5 生成逻辑或其他动态系统 ID 生成逻辑

#### Scenario: 系统 ID 固化常量

- **WHEN** 代码定义 `SuperAdminRoleID`、`BootstrapSuperAdminUserID` 或任一 baseline permission ID
- **THEN** 常量 MUST 位于 `user-service/internal/shared/rbacbaseline/ids.go`
- **AND** 每个常量 MUST 是手写固化的 UUID 字符串，MUST NOT 由 UUIDv5、`go:generate` 或运行时代码生成
- **AND** 每个常量 MUST 匹配 `00000000-0000-0000-0000-TTMMSSSSSSSS` 保留格式
- **AND** `TT` MUST 表示实体类型，其中 `01` 为系统用户、`02` 为系统角色、`03` 为系统权限、`09` 为测试、fixture 或诊断预留
- **AND** 系统用户和系统角色的 `MM` MUST 为 `00`
- **AND** 系统权限的 `MM` MUST 使用当前真实进入 `DefaultPermissions()` 的权限模块连续编号，当前编号 MUST 为 `01=user`、`02=permission`、`03=role`、`04=user-role`、`05=role-permission`
- **AND** `SSSSSSSS` MUST 是同一 `TT+MM` 下从 `00000001` 开始递增的序号，MUST NOT 使用 `00000000`
- **AND** `ids.go` MUST 集中记录 type、module 和 sequence 编码规则
- **AND** 每个常量注释 MUST 记录稳定 semantic

#### Scenario: 权限模块编号追加

- **WHEN** 后续新增权限模块首次进入 `rbacbaseline.DefaultPermissions()`
- **THEN** 系统 MUST 按该模块首次进入 baseline 的顺序分配下一个可用正式 `MM`
- **AND** 已发布模块编号 MUST NOT 因后续新增、删除或重排权限而修改
- **AND** 系统 MUST NOT 提前为 `auth`、`audit`、`tenant` 或其他尚未进入 `DefaultPermissions()` 的未来权限模块预分配编号
- **AND** `90` 至 `99` 的权限模块编号 MUST 只用于测试、fixture 或诊断预留，MUST NOT 用于生产 baseline

#### Scenario: 权限基线引用系统 ID 常量

- **WHEN** `rbacbaseline.DefaultPermissions()` 返回权限基线
- **THEN** 每个 `PermissionSpec.PermissionID` MUST 引用 `rbacbaseline` 中的 permission ID 常量
- **AND** `DefaultPermissions()` MUST NOT 内联 UUID 字符串
- **AND** `DefaultRoles()` 和 `DefaultRolePermissions()` MUST 引用 `rbacbaseline` 中的角色或权限 ID 常量
- **AND** 系统 MUST 通过测试校验所有默认权限 ID 和默认绑定引用均存在且没有重复

#### Scenario: 一次性超级管理员引导输入

- **WHEN** 运维执行 `aegiscore-user-service rbac bootstrap-super-admin --username <name> --nickname <nickname> --password-env <env>`
- **THEN** 系统 MUST 要求 `--username` 必填且无默认值，并将 username trim 后转为小写
- **AND** `--nickname` MUST 可选，trim 后为空时 MUST 使用归一化 username
- **AND** `--password-env` 默认 MUST 为 `ADMIN_BOOTSTRAP_PASSWORD`，密码 MUST 只从该环境变量读取
- **AND** 密码 MUST NOT trim，首尾空格 MUST 作为密码内容参与校验和哈希
- **AND** 密码长度 MUST 为 12 至 72 字节
- **AND** 命令行 MUST NOT 提供直接传递密码、user ID、reset、force、reuse 或 reactivate 的参数

#### Scenario: 固定 bootstrap 标识

- **WHEN** 系统执行超级管理员首次引导
- **THEN** 系统 MUST 使用 `user-service/internal/shared/rbacbaseline.BootstrapSuperAdminUserID` 作为固定用户 ID
- **AND** 固定用户 ID MUST 由代码定义，MUST NOT 通过 CLI 参数、环境变量或配置覆盖
- **AND** 系统 MUST 使用 `rbacbaseline.SuperAdminRoleID` 作为超级管理员角色 ID
- **AND** bootstrap application MUST NOT 在自身 package 中私有定义 bootstrap 系统用户 ID

#### Scenario: 事务性首次引导

- **WHEN** bootstrap store 执行首次超级管理员引导
- **THEN** 系统 MUST 在同一个 PostgreSQL 事务中获取固定 transaction advisory lock、查询超级管理员角色、检查固定用户 ID、检查 username、创建固定 ID 用户并创建用户角色绑定
- **AND** 超级管理员角色 MUST 存在、`is_system=true` 且 `active=true`
- **AND** 固定用户 ID 查询 MUST 包含软删除用户，MUST NOT 添加 `deleted_at IS NULL`
- **AND** username 占用检查 MUST 覆盖正常用户和软删除用户
- **AND** bootstrap 用户状态 MUST 为 `identity.UserStatusMustChangePassword`
- **AND** 用户密码 MUST 使用应用层传入的 bcrypt hash
- **AND** 任一步失败 MUST 回滚整个事务，不得留下已创建但没有角色的用户、已绑定角色但用户状态错误的记录或只有用户没有完整 bootstrap 结果的部分状态
- **AND** 唯一约束冲突 MUST 映射成稳定应用错误，MUST NOT 直接暴露 Ent 或 PostgreSQL 错误

#### Scenario: 重复执行拒绝

- **WHEN** 数据库中存在固定 `BootstrapSuperAdminUserID` 对应用户
- **THEN** `rbac bootstrap-super-admin` MUST 拒绝执行并返回非零退出码
- **AND** 稳定公开错误消息 MUST 为 `super admin bootstrap has already been completed`
- **AND** 无论该用户是否软删除、是否禁用、是否仍有超级管理员角色或 username 是否被修改，系统都 MUST 视为已完成引导
- **AND** 命令 MUST NOT 尝试修复、复用、重置或重新绑定该用户

#### Scenario: 后续超级管理员授权

- **WHEN** 首次 bootstrap 已完成且需要授权其他超级管理员
- **THEN** 系统 MUST 通过现有在线用户角色绑定 API 完成授权
- **AND** 在线流程 MUST 负责权限校验、policy version 发布和缓存收敛
- **AND** 系统 MUST NOT 允许再次运行 bootstrap 创建其他超级管理员
- **AND** 系统 MUST NOT 提供离线密码重置、离线超级管理员恢复或 `recover-super-admin` 入口
- **AND** 如果所有超级管理员均不可用，本方案 MUST 只允许 DBA 人工介入或重新初始化数据库

#### Scenario: 删除旧超级管理员命令

- **WHEN** 运维或测试调用 `rbac create-super-admin`、`rbac assign-super-admin`、`--reset-password` 或 `ADMIN_RESET_PASSWORD`
- **THEN** 系统 MUST 拒绝或忽略旧入口，使旧命令和旧 flag 无法作为公开 CLI 调用
- **AND** 系统 MUST NOT 保留旧命令别名、双版本 CLI 共存或旧数据自动恢复行为

#### Scenario: 离线命令不等同在线刷新

- **WHEN** HTTP 副本运行期间执行 seed 或 bootstrap-super-admin
- **THEN** 命令 MUST 只修改持久化数据并 MUST NOT 宣称已触发运行期 policy refresh
- **AND** 运维 MUST 滚动重启副本或触发在线 RBAC 刷新使运行实例收敛

#### Scenario: 系统 ID 一致性门禁

- **WHEN** 执行 user-service 测试
- **THEN** 测试 MUST 校验所有系统 ID 常量均可通过 UUID parser 解析
- **AND** 每个系统 ID 常量 MUST 匹配 `^00000000-0000-0000-0000-[0-9]{12}$`
- **AND** 每个系统 ID 常量最后 12 位中的 `TT` MUST 匹配测试登记的类型编号
- **AND** 每个系统 ID 常量最后 12 位中的 `MM` MUST 匹配测试登记的模块编号
- **AND** 每个系统 ID 常量最后 8 位 sequence MUST NOT 为 `00000000`
- **AND** 全部系统 ID MUST 全局唯一
- **AND** 全部 baseline permission ID MUST 登记在 `registeredPermissionIDs()` 并被默认权限和默认绑定校验覆盖

#### Scenario: 已发布系统 ID 不可复用

- **WHEN** 系统 ID 已随代码发布或对应 baseline 项被删除
- **THEN** 后续变更 MUST NOT 修改该 ID 的字符串值
- **AND** 后续变更 MUST NOT 将已删除 baseline 项的 ID 复用于其他系统用户、系统角色或系统权限

### Requirement: RBAC 错误与统一 HTTP 契约

permission、role 和 binding domain MUST 返回携带稳定 HTTP status、共享业务 code、公开 message 和 `Reason` 的应用错误。HTTP transport MUST 通过共享 `response.Fail` 直接渲染业务错误，MUST NOT 维护 feature 专用 sentinel-to-HTTP mapper；直接或包装返回的应用错误 MUST 保留 `errors.Is` 语义。permission 查询端只保留输入无效和权限不存在等仍可达错误，不得保留只服务于权限创建、更新、启停或系统权限保护的公开错误契约。

#### Scenario: 目录与绑定错误稳定映射

- **WHEN** permission 查询或 role permission lookup 返回权限不存在或输入无效错误
- **THEN** 系统 MUST 分别使用 `404 Not Found` 或 `400 Bad Request`，且 `Reason` MUST 分别为 `permission_not_found` 或 `permission_invalid`
- **AND** permission feature MUST NOT 暴露权限已存在或系统权限保护作为公开 HTTP 写错误
- **WHEN** role feature 返回角色已存在、角色不存在、输入无效、系统角色保护或角色停用错误
- **THEN** 系统 MUST 分别使用 `409 Conflict`、`404 Not Found`、`400 Bad Request`、`409 Conflict`、`409 Conflict`，且 `Reason` MUST 分别为 `role_already_exists`、`role_not_found`、`role_invalid`、`system_role_protected`、`role_inactive`
- **WHEN** 用户角色或角色权限增量绑定返回绑定已存在或绑定不存在错误
- **THEN** 系统 MUST 分别返回 `409 Conflict` 或 `404 Not Found`，并使用对应稳定 `Reason`

#### Scenario: 跨 feature 错误透传和统一出口

- **WHEN** role 流程收到 `identity.ErrUserNotFound` 或 permission 的不存在错误
- **THEN** role HTTP transport MUST 通过共享 response helper 保留错误自身的 status、code 和 message
- **AND** role transport MUST NOT 复制 identity 或 permission 错误映射
- **WHEN** permission query 或 role command/query 返回业务错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** transport MUST NOT 调用或保留 `toPermissionHTTPError`、`toRoleHTTPError` 或等价 mapper

### Requirement: RBAC 可观测性

系统 MUST 为 RBAC 授权判定提供低基数 metrics，并使用显式注入的 logger 记录加载和同步异常。观测失败 MUST NOT 改变授权或策略同步结果。RBAC policy sync Redis key prefix、Pub/Sub channel 和 metrics `service` label MUST 使用 `aegiscore-user-service`，旧 `aegiscore-user-services` prefix 或兼容 channel MUST NOT 被读取、发布或订阅。生产运行时 MUST NOT 装配只服务于公开 route diff 的 query、scanner 或 metrics。

#### Scenario: 授权 metrics 的低基数与敏感数据约束

- **WHEN** permission authorization service 完成一次 RBAC Enforce 判定
- **THEN** counter MUST 记录 `result="allow"`、`result="deny"` 或 `result="error"`
- **AND** histogram MUST 记录本次判定耗时
- **AND** 标签 MUST 只使用 `result`、HTTP method 和 route template
- **AND** 指标 MUST NOT 包含用户、角色、权限、token、trace、IP、账号、Redis key、SQL、原始错误或 raw path
- **AND** user-service 默认 `service` label MUST 为 `aegiscore-user-service`

#### Scenario: 日志观测和 route diff 移除

- **WHEN** role 或 permission application、policy loader、watcher、cache 或 adapter 需要记录日志
- **THEN** logger MUST 由 constructor 显式注入或由调用方 context 提供
- **AND** 日志 MUST 使用稳定低基数字段并 MUST NOT 记录 token、SQL、Redis key 或原始 policy 数据
- **WHEN** user-service 组装生产 permission 模块
- **THEN** 系统 MUST NOT 注册公开 route diff handler 或装配专用于 route diff 的 metrics
- **AND** 路由基线一致性 MUST 由 CI/测试失败表达，不得自动修改权限投影

#### Scenario: RBAC policy sync key 和 channel

- **WHEN** permission Redis adapter 生成 policy version key 或 policy refresh channel
- **THEN** key 和 channel prefix MUST 来自当前 `app.name` 并归一化为 `aegiscore-user-service`
- **AND** adapter MUST NOT 读取、发布、订阅或迁移旧 `aegiscore-user-services` prefix 下的 policy version key 或 Pub/Sub channel
- **AND** 发布后副本收敛 MUST 依赖新 prefix 下的 version key、Pub/Sub channel 和周期性补偿

### Requirement: RBAC 架构、装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或仅服务于公开 route diff 的生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应 composition、transport 或 infrastructure 边界。RBAC 自有 watcher、cache、user-role resolver 和 policy 投影资源 MUST 显式启动、停止和回滚。permission composition MUST 用单一运行时聚合对象表达对外稳定 RBAC runtime 组件集合，并避免为 named/private 到 public 投影保留重复样板。

#### Scenario: 分层和最小依赖

- **WHEN** role 或 permission application service 在单元测试或非 Fx 调用方中构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明仅服务于 DI 的 tag
- **AND** 消费侧 application MUST 定义最小 port 并由相邻 feature 或 integration adapter 实现，feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport
- **AND** role 仍使用的 permission lookup MUST NOT 因删除 permission command 而被移除

#### Scenario: bootstrap application 和 store 边界

- **WHEN** 实现超级管理员 bootstrap 应用服务
- **THEN** 服务 MUST 位于 `user-service/internal/features/role/application/bootstrap/`，并通过最小 `BootstrapStore` port 调用持久化能力
- **AND** application 层 MUST 负责校验和归一化输入、校验 bootstrap 密码策略、哈希密码、使用固定 bootstrap user ID 和固定 super admin role ID
- **AND** application 层 MUST NOT 导入 Ent predicate、HTTP transport、Gin、Fx、SQL、Redis 或 datastore concrete implementation
- **AND** PostgreSQL adapter MUST 位于 role infrastructure 边界并只实现 bootstrap application 拥有的最小 port

#### Scenario: framework-neutral adapter 和 composition 边界

- **WHEN** 构造 role store、permission store、policy loader、Casbin engine、Redis policy store、watcher、cache 或 adapter
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options
- **AND** constructor MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、named result 或 group result
- **AND** 具名 `primary_db`、`cache_redis`、optional、group 或生命周期依赖选择 MUST 留在 feature composition 边界
- **AND** public provider MUST 只暴露 controller、authorizer、health/status、运行时聚合对象和 application port 等稳定 contract，父 module MUST NOT 消费 feature infrastructure concrete implementation

#### Scenario: RBAC runtime 聚合投影

- **WHEN** permission composition 对外提供授权、policy health、policy watcher status、policy change notifier、policy initializer、policy watcher runner 或 user-role resolver lifecycle
- **THEN** composition MUST 通过单一 permission runtime 聚合对象表达这些组件属于同一组 RBAC runtime
- **AND** 聚合对象字段 MUST 来自已经构造完成的稳定接口或 composition 私有 lifecycle contract
- **AND** 聚合对象 MUST NOT 在内部重新构造 Casbin engine、policy store、watcher、version tracker、cache、resolver、Redis client 或 Ent client
- **AND** application/domain MUST NOT 依赖该聚合对象

#### Scenario: 有状态资源单实例多视图

- **WHEN** composition 需要同时提供 authorization、policy reload、policy health、policy store 或 publisher 等接口视图
- **THEN** composition MUST 为同一有状态 adapter 构造一个实例并通过普通 Go 赋值暴露所需端口
- **AND** 系统 MUST NOT 为不同接口视图重复构造有状态 engine、store、version tracker、watcher 或 cache
- **AND** watcher 的状态视图和运行器视图 MUST 指向同一 watcher 实例

#### Scenario: 必需同步依赖不可降级

- **WHEN** 角色、角色权限或用户角色写侧服务装配完成
- **THEN** 服务 MUST 具备可用的 policy change notifier
- **AND** 缺少 notifier 或其他必需安全 collaborator 时 constructor MUST 返回明确 error 并拒绝装配，MUST NOT panic
- **AND** 系统 MUST NOT 以 no-op、nil fallback 或兼容 wrapper 静默跳过 policy reload、Redis policy version 或 watcher 同步语义

#### Scenario: watcher、cache 和 lifecycle

- **WHEN** user-service 启停 Redis policy watcher 和 user-role resolver/cache
- **THEN** `NewWatcher` MUST 只构造 watcher 对象，MUST NOT 启动 goroutine、订阅 Redis 或执行版本补偿循环
- **AND** user-role resolver/cache 的 Fx result MUST 显式提供同时具备 `Start(context.Context) error` 与 `Close() error` 的 lifecycle 视图，lifecycle hook MUST NOT 通过关闭接口的 type assertion 探测启动能力
- **AND** lifecycle hook MUST 在启动阶段先调用 user-role resolver/cache 的 `Start(ctx)`，再执行初始 policy 加载并启动 watcher
- **AND** user-role resolver/cache 启动失败时 MUST 返回启动错误，且 MUST NOT 执行初始 policy 加载或启动 watcher
- **AND** `Start()` 和 `Stop(ctx)` MUST 幂等，`Stop(ctx)` MUST 取消内部 context，并在调用方 context 限制内等待循环退出
- **AND** `Stop(ctx)` 超时时 MUST 返回 context 相关错误，并保持后续重复停止安全
- **AND** 启动失败或服务停止时已启动 watcher MUST 被停止，已创建 cache MUST 幂等关闭
- **AND** watcher stop 和 cache close 同时失败时单个 lifecycle hook MUST 保留全部 cause，且 cache close MUST 在 watcher stop 返回错误时仍被执行

#### Scenario: 共享资源所有权和 fail-closed

- **WHEN** RBAC 关闭 watcher、cache、store 或 resolver
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源
- **AND** 关闭后授权语义 MUST 继续 fail-closed，不得因为本地资源不可用而产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置、权限基线或 key schema 下沉到 `common`

### Requirement: RBAC 运行时 ID 与系统 ID 分离

系统 MUST 区分普通运行时业务 ID 和系统内置稳定 ID。普通运行时业务实体 MUST 继续使用 `common/runtime/id.NewUUID()` 生成 UUID v7；RBAC 系统基线和 bootstrap ID MUST 使用 `rbacbaseline` 中的手写保留 UUID 固化常量。

#### Scenario: 普通业务实体创建
- **WHEN** 系统创建普通用户、普通角色或其他运行时业务数据
- **THEN** 创建路径 MUST 使用当前运行时 ID 生成策略生成新 ID
- **AND** 普通运行时创建路径 MUST NOT 复用 RBAC 系统 ID 常量
- **AND** 普通业务用户、普通角色和运行时创建数据 MUST NOT 使用 `00000000-0000-0000-0000-TTMMSSSSSSSS` 系统保留 ID 格式

#### Scenario: 系统基线禁止随机 ID
- **WHEN** 系统创建或更新超级管理员角色、bootstrap 用户、baseline permission 或默认角色权限绑定
- **THEN** 系统 MUST 使用 `rbacbaseline` 中的固化系统 ID 常量
- **AND** 系统 MUST NOT 使用 `common/runtime/id.NewUUID()` 为这些系统基线生成 ID
- **AND** 系统 MUST NOT 在正式运行路径动态调用 UUIDv5、随机 UUID 或其他生成逻辑来替代固化常量

### Requirement: 默认系统角色权限基线维护

系统 MUST 在 `internal/shared/rbacbaseline` 中集中维护默认系统角色及其默认权限绑定。`DefaultRoles()`、`DefaultPermissions()` 和 `DefaultRolePermissions()` 的公开函数签名 MUST 保持稳定；默认角色权限绑定 MUST 只引用 `DefaultPermissions()` 中的稳定 `PermissionID`。

#### Scenario: 当前默认基线行为保持不变

- **WHEN** 代码调用 `rbacbaseline.DefaultRoles()`
- **THEN** 系统 MUST 仍然只返回内置超级管理员角色作为当前默认角色
- **WHEN** 代码调用 `rbacbaseline.DefaultRolePermissions()`
- **THEN** 系统 MUST 仍然返回超级管理员角色到全部默认权限的绑定
- **AND** 绑定集合 MUST 不包含重复的 `RoleID` 与 `PermissionID` 组合

#### Scenario: 默认角色绑定引用已知基线

- **WHEN** 系统展开默认角色权限绑定
- **THEN** 每条绑定的 `RoleID` MUST 引用 `DefaultRoles()` 返回的已知默认角色
- **AND** 每条绑定的 `PermissionID` MUST 引用 `DefaultPermissions()` 返回的已知默认权限

#### Scenario: 未来默认角色显式维护权限

- **WHEN** 后续新增非超级管理员默认系统角色
- **THEN** 该角色的默认权限 MUST 在角色 catalog block 中显式列出 `PermissionID`
- **AND** 系统 MUST NOT 按 `Module`、model、read/write、路由前缀或其他粗粒度集合自动推导该角色的默认权限
- **AND** 系统 MUST NOT 为了表达默认角色权限引入 `PermissionSet` 别名层

#### Scenario: 超级管理员全量绑定例外

- **WHEN** 系统展开超级管理员默认权限绑定
- **THEN** 系统 MUST 允许超级管理员使用内部 helper 自动绑定 `DefaultPermissions()` 中的全部权限
- **AND** 该自动全量绑定例外 MUST NOT 扩展到其他默认系统角色
