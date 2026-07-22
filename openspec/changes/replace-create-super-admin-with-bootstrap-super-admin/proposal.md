## Why

当前 `rbac create-super-admin` 可重复执行并支持复用、重置密码和离线绑定，容易把首次系统引导、管理员恢复和后续授权混在同一运维入口中，扩大了离线高权限操作面。
本变更将超级管理员初始化收敛为一次性的 `rbac bootstrap-super-admin`，用代码固定的 bootstrap 用户 UUID 作为唯一完成标识，使全新数据库的首次引导可审计、可预测且不会隐式兼容旧数据或旧命令。

## What Changes

- **BREAKING** 删除 `rbac create-super-admin`、`rbac assign-super-admin`、`--reset-password`、`ADMIN_RESET_PASSWORD`、旧的 `ADMIN_USERNAME` 默认值和对应 Makefile 入口。
- **BREAKING** 新增 `aegiscore-user-service rbac bootstrap-super-admin`，仅支持全新数据库的首次超级管理员引导，不兼容旧超级管理员数据、旧命令别名或 marker 回填。
- 新增代码固定的 bootstrap 用户 UUID `00000000-0000-0000-0000-000000000002`，禁止通过 CLI 参数覆盖，并继续使用 `rbacbaseline.SuperAdminRoleID` 作为超级管理员角色标识。
- bootstrap 命令要求显式 `--username`，归一化为 trim 后小写；`--nickname` 可选，trim 后为空则回退为归一化 username；密码只从 `--password-env` 指定的环境变量读取，默认 `ADMIN_BOOTSTRAP_PASSWORD`，不做 trim，长度限制为 12 至 72 字节。
- bootstrap 只允许创建固定 bootstrap 用户一次；只要该用户 ID 存在，包括软删除、禁用、角色丢失或 username 被修改，都必须拒绝再次执行并返回稳定错误 `super admin bootstrap has already been completed`。
- bootstrap 用户必须以 `identity.UserStatusMustChangePassword` 创建，首次登录只获得 password-change token，后续强制改密继续复用现有认证撤销、token version、Redis 投影、本地缓存失效和 refresh session 撤销能力。
- bootstrap 持久化必须在同一 PostgreSQL 事务中完成，并使用固定 PostgreSQL transaction advisory lock，验证超级管理员角色存在、为 system role 且启用，检查固定用户 ID 与 username 在正常和软删除用户中均未占用，然后创建用户和角色绑定。
- 不新增 `rbac_bootstrap_states` 表，不新增 Ent schema 或 Atlas migration，不提供 `recover-super-admin`、reset、force、reuse、reactivate 或 user ID 参数。
- 后续超级管理员授权只能通过现有在线用户角色绑定 API 完成，由在线流程负责权限校验、policy version 发布和缓存收敛。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 将超级管理员维护从可重复离线创建/绑定改为一次性 bootstrap，并定义固定用户 ID、事务性持久化、重复执行语义、错误契约和后续授权边界。
- `auth-session-management`: 明确 bootstrap 用户以强制改密状态创建，首次登录和改密必须复用现有 password-change token、条件凭据更新和会话撤销语义。
- `delivery-operations`: 调整 Makefile 入口和全新数据库部署顺序，删除旧超级管理员命令目标与环境变量，新增 `bootstrap-super-admin` 发布步骤。

## Impact

- 影响代码：`user-service/cmd/rbac.go`、`user-service/internal/features/role/application/bootstrap/`、`user-service/internal/features/role/infrastructure/postgres/`、role provider/wiring、相关单元测试、集成测试和 E2E。
- 影响 CLI：删除 `rbac create-super-admin`、`rbac assign-super-admin` 和旧 flags/env，新增 `rbac bootstrap-super-admin --username --nickname --password-env`。
- 影响交付入口：根 `Makefile` 与 `user-service/Makefile` 删除 `create-super-admin` 相关目标，根目录新增带服务名前缀的 `user-service-bootstrap-super-admin`，服务目录新增 `bootstrap-super-admin`。
- 影响安全：离线超级管理员操作面缩小为全新数据库一次性创建固定 bootstrap 用户，后续授权必须走在线 RBAC 授权路径。
- 影响数据库：无 Ent schema 变化，无新增业务表和 Atlas migration；但 PostgreSQL adapter 需要使用事务、软删除包含查询、唯一约束错误映射和 transaction advisory lock。
- 影响发布：仅支持 `Atlas migration -> rbac seed -> bootstrap-super-admin -> 启动 HTTP 副本 -> 初始管理员强制改密`，不支持旧库原地升级、旧数据识别、命令兼容或自动恢复已有管理员。
