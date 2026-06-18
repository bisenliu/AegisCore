# RBAC 规格

## 需求

### 需求：角色管理
角色功能必须拥有角色生命周期、用户角色绑定、角色权限绑定、系统角色保护和角色查询用例。

#### 场景：为角色绑定权限
Given 角色需要绑定权限
When 角色 application 校验权限 ID
Then 必须通过权限 application 查询端口校验，不得导入权限 infrastructure。

#### 场景：停用角色
Given 角色已停用
When 加载授权策略
Then 通过该角色形成的绑定不得授予访问权限。

### 需求：权限目录
权限功能必须拥有权限生命周期、权限查询、有效权限、路由差异诊断、授权包装器、Gin RBAC 中间件、Casbin 加载器/执行器/重载和策略同步编排。

#### 场景：停用权限
Given 权限已停用
When 查询有效权限或加载 Casbin 策略
Then 该权限不得授予访问权限。

#### 场景：路由差异诊断
Given 已注册受保护路由与正式权限目录存在差异
When 查询路由差异
Then 只能返回 missing/stale 差异，不得创建权限、修改权限状态或绑定角色。

#### 场景：可授权路由发现
Given 扫描 Gin 路由
When 构造路由差异诊断输入
Then 必须排除 `OPTIONS`、`/api/v1/` 外路径和认证公开/会话控制路由；application 层不得直接依赖 Gin engine。

### 需求：授权契约
受保护业务路由必须先完成 JWT 认证，再执行 RBAC 授权。Casbin subject/object/action 必须分别使用 `user:<user_uuid>`、`role:<role_uuid>`、Gin 路由模板和 HTTP method。

#### 场景：用户无角色
Given 已认证用户没有有效角色绑定
When 访问 RBAC 保护路由
Then 必须拒绝访问。

#### 场景：超级管理员通配授权
Given 用户拥有来自 `rbacbaseline` 的内置超级管理员角色
When 加载策略
Then 必须按基线补充通配策略，使其可访问受保护业务接口。

#### 场景：Casbin policy 权威来源
Given 需要加载 RBAC Casbin policy
When policy loader 从持久化层构造策略
Then 策略必须由启用角色、启用权限、角色权限绑定和用户角色绑定派生，不得以独立 `casbin_rules` 表作为业务权威来源。

#### 场景：Casbin subject 稳定格式
Given 角色参与授权
When 构造 Casbin policy 或执行授权
Then 角色 subject 必须使用 `role:<role_uuid>`，不得依赖 `roles.code`；用户身份解析必须排除已软删除用户。

#### 场景：超级管理员 wildcard policy
Given 加载内置超级管理员策略
When 构造 Casbin policy
Then 必须基于 `internal/shared/rbacbaseline` 中稳定的超级管理员角色 ID 补充 wildcard policy，不得在 role 或 permission 功能内重复定义超级管理员常量。

### 需求：策略同步和 seed 工作流
在线 RBAC 写操作必须触发本实例策略重载，并通过 Redis version/Pub/Sub 通知其他副本。其他副本必须通过 Pub/Sub 和版本补偿重载。授权热路径不得每请求读取 Redis 做强一致校验。

#### 场景：在线角色绑定变更
Given 用户角色绑定通过 HTTP API 变更
When 写操作成功
Then 本实例必须执行策略刷新编排，并通知其他副本。

#### 场景：在线权限策略变更
Given 在线 RBAC 管理接口修改权限、角色启停或角色权限绑定
When 数据库写入成功
Then 本实例必须执行 policy reload，并通过 Redis policy version 和 Pub/Sub 通知其他副本；其他副本必须通过 Pub/Sub 和周期性版本补偿感知变更。

#### 场景：用户角色绑定缓存失效
Given 在线接口修改用户角色绑定
When 变更已提交
Then 本实例和其他副本至少必须使受影响用户的角色解析缓存失效；无需重建不依赖用户绑定的 permission policy。

#### 场景：授权热路径
Given 业务请求进入 RBAC 授权中间件
When 执行授权
Then 授权只能使用本实例内存 Casbin enforcer 和本地可用的用户角色解析结果，不得每请求读取 Redis policy version 做强一致门控。

#### 场景：离线 RBAC 运维入口
Given 执行 `rbac seed`、`rbac assign-super-admin` 或 `rbac create-super-admin`
When HTTP 副本已经运行
Then 运维必须滚动重启副本或触发一次在线策略刷新；这些离线命令不得被视为运行期策略同步。

#### 场景：离线 RBAC seed 的授权边界
Given 执行 `rbac seed`
When 初始化系统角色、系统权限和默认系统绑定
Then seed 不得自动创建真实业务用户、为任意业务用户分配超级管理员角色，也不得自动授权 custom role；超级管理员分配必须通过 `rbac assign-super-admin --user-id <uuid>` 或 `rbac create-super-admin` 显式执行。

#### 场景：创建超级管理员账号
Given 执行 `rbac create-super-admin`
When 需要创建或复用管理员用户并绑定内置超级管理员角色
Then 命令必须与 `rbac seed` 分离执行，必须通过环境变量读取密码，不得内置默认密码；当管理员用户已存在时默认不得重置密码，只有显式传入 `--reset-password` 时才允许重置密码并恢复正常状态。

#### 场景：运行中执行离线 RBAC 命令
Given 已有 HTTP 副本正在运行
When 执行 `rbac seed`、`rbac assign-super-admin` 或 `rbac create-super-admin`
Then 命令只修改持久化数据，不得被视为运行期 policy refresh；运维必须滚动重启副本或触发在线 RBAC 刷新。
