## MODIFIED Requirements

### Requirement: RBAC 系统数据与运维 CLI

系统 MUST 提供带服务上下文的 `rbac seed` 和一次性 `rbac bootstrap-super-admin` 命令，用于维护系统角色、代码定义权限投影、默认绑定和全新数据库的首次超级管理员引导。系统角色、系统权限、默认绑定和 bootstrap 用户 ID MUST 由 `internal/shared/rbacbaseline` 以 UUID v5 生成后固化常量定义，全部权限定义 MUST 只来自 `rbacbaseline.DefaultPermissions()`。RBAC 运维 CLI MUST 通过 `aegiscore-user-service` 根命令调用，旧 `aegiscore-user-services` 根命令 MUST NOT 作为 RBAC 兼容入口保留。

#### Scenario: 初始化系统基线

- **WHEN** 运维执行 `aegiscore-user-service rbac seed`
- **THEN** 系统 MUST 幂等创建或更新基线角色、权限投影和绑定并输出变更统计
- **AND** 系统角色 MUST 保持系统数据标记，Permission MUST NOT 包含 `Active` 或 `IsSystem`
- **AND** seed MUST NOT 创建业务用户、自动分配超级管理员或自动删除基线之外的权限记录
- **AND** 非 seed 的公开 HTTP 路径 MUST NOT 创建、修改或启停权限
- **AND** seed 写入的系统角色和权限 ID MUST 引用 `rbacbaseline` 中的固化常量，MUST NOT 在 seed 运行时调用 `id.NewUUID()` 或动态 UUID v5 生成逻辑

#### Scenario: 系统 ID 固化常量

- **WHEN** 代码定义 `SuperAdminRoleID`、`BootstrapSuperAdminUserID` 或任一 baseline permission ID
- **THEN** 常量 MUST 位于 `user-service/internal/shared/rbacbaseline` 边界
- **AND** 每个常量 MUST 是由 `UUIDv5(SystemIDNamespace, semantic name)` 生成后固化的 UUID 字符串
- **AND** 常量注释 MUST 记录对应 semantic name
- **AND** semantic name MUST 使用稳定业务语义，例如 `role:super-admin`、`user:bootstrap-super-admin` 或 `permission:<resource>:<action>`
- **AND** semantic name MUST NOT 使用项目名、服务名、HTTP path、中文展示文案或 Go symbol

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
- **THEN** 测试 MUST 校验 `SystemIDNamespace` 和所有系统 ID 常量均可解析
- **AND** 每个系统 ID 常量的 UUID 版本 MUST 为 5
- **AND** 每个系统 ID 常量 MUST 等于 `uuid.NewSHA1(SystemIDNamespace, []byte(semantic name))` 的结果
- **AND** 全部系统 ID MUST 无重复

## ADDED Requirements

### Requirement: RBAC 运行时 ID 与系统 ID 分离

系统 MUST 区分普通运行时业务 ID 和系统内置稳定 ID。普通运行时业务实体 MUST 继续使用 `common/runtime/id.NewUUID()` 生成 UUID v7；RBAC 系统基线和 bootstrap ID MUST 使用 `rbacbaseline` 中的 UUID v5 固化常量。

#### Scenario: 普通业务实体创建
- **WHEN** 系统创建普通用户、普通角色或其他运行时业务数据
- **THEN** 创建路径 MUST 使用当前运行时 ID 生成策略生成新 ID
- **AND** 普通运行时创建路径 MUST NOT 复用 RBAC 系统 ID 常量

#### Scenario: 系统基线禁止随机 ID
- **WHEN** 系统创建或更新超级管理员角色、bootstrap 用户、baseline permission 或默认角色权限绑定
- **THEN** 系统 MUST 使用 `rbacbaseline` 中的固化系统 ID 常量
- **AND** 系统 MUST NOT 使用 `common/runtime/id.NewUUID()` 为这些系统基线生成 ID
- **AND** 系统 MUST NOT 在正式运行路径动态调用 UUID v5 生成逻辑来替代固化常量
