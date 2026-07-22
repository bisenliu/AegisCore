## 1. CLI 与入口清理

- [x] 1.1 删除 `user-service/cmd/rbac.go` 中的 `create-super-admin`、`assign-super-admin`、`--reset-password`、`ADMIN_RESET_PASSWORD` 和 `ADMIN_USERNAME` 默认值逻辑。
- [x] 1.2 新增 `rbac bootstrap-super-admin` Cobra 命令，支持 `--config`、必填 `--username`、可选 `--nickname` 和默认 `ADMIN_BOOTSTRAP_PASSWORD` 的 `--password-env`。
- [x] 1.3 确认命令不提供密码明文参数、user ID、reset、force、reuse 或 reactivate 参数，并让旧命令和旧 flags 无法调用。
- [x] 1.4 更新 CLI help、命令错误传播和非零退出码测试，覆盖旧命令删除、新命令必填参数和稳定错误输出。

## 2. Role Bootstrap Application

- [x] 2.1 新增 `user-service/internal/features/role/application/bootstrap/`，包含 `service.go`、`ports.go`、`errors.go` 和 `service_test.go`。
- [x] 2.2 定义 `BootstrapSuperAdminUserID = "00000000-0000-0000-0000-000000000002"`，并在 service 中固定使用 `rbacbaseline.SuperAdminRoleID`。
- [x] 2.3 定义最小 `BootstrapStore` port、`BootstrapSuperAdminInput` 和 `BootstrapSuperAdminResult`，避免 application 层导入 Ent predicate、HTTP transport、Gin、Fx、SQL 或 Redis concrete implementation。
- [x] 2.4 实现输入校验和归一化：username trim 后转小写、空 nickname 回退 username、密码从指定环境变量读取且不 trim、密码长度为 12 至 72 字节。
- [x] 2.5 实现 bcrypt hash 后调用 store，并将固定 user ID、super admin role ID、username、nickname、password hash 和 `identity.UserStatusMustChangePassword` 传入持久化层。
- [x] 2.6 添加 application 单元测试，覆盖首次 bootstrap 成功、固定 UUID、MustChangePassword 状态、username 归一化、nickname 回退、密码首尾空格保留、短密码失败和环境变量缺失失败。

## 3. PostgreSQL Bootstrap Store

- [x] 3.1 在 `user-service/internal/features/role/infrastructure/postgres/bootstrap_store.go` 新增专用 adapter，实现 bootstrap application 的 `BootstrapStore` port。
- [x] 3.2 在同一 PostgreSQL 事务中获取固定 transaction advisory lock，确保并发执行时最多一个命令成功。
- [x] 3.3 在事务中按 `rbacbaseline.SuperAdminRoleID` 查询角色，并校验角色存在、`is_system=true` 且 `active=true`。
- [x] 3.4 查询固定 `BootstrapSuperAdminUserID` 时包含软删除用户，不添加 `deleted_at IS NULL`，存在时返回 `ErrSuperAdminAlreadyBootstrapped`。
- [x] 3.5 检查 username 是否被任何正常或软删除用户占用，占用时返回 `ErrBootstrapUsernameAlreadyExists`。
- [x] 3.6 创建固定 ID 用户、写入应用层传入的 bcrypt hash 和 `identity.UserStatusMustChangePassword`，再创建用户与超级管理员角色绑定。
- [x] 3.7 将唯一约束冲突和事务内前置条件失败映射为稳定应用错误，不直接暴露 Ent 或 PostgreSQL 原始错误。
- [x] 3.8 添加 store 集成测试，覆盖固定用户 ID 已存在、固定用户 ID 软删除仍拒绝、username 被正常用户占用、username 被软删除用户占用、角色不存在、角色非 system、角色停用、用户创建失败回滚、角色绑定失败回滚和并发执行最多一个成功。

## 4. Wiring 与交付入口

- [x] 4.1 更新 role/provider wiring，使 `rbac bootstrap-super-admin` 使用正式 config、主 PostgreSQL/Ent 资源、bootstrap store 和 bootstrap application service。
- [x] 4.2 确认 bootstrap CLI 不装配或直接调用 auth 撤销逻辑，用户首次登录和强制改密继续由 auth feature 现有流程处理。
- [x] 4.3 更新根 `Makefile` 和 `user-service/Makefile`，删除 `user-service-create-super-admin`、根目录 `bootstrap-super-admin`、`create-super-admin` 和 `ADMIN_RESET_PASSWORD`，根目录新增 `user-service-bootstrap-super-admin`，服务目录保留 `bootstrap-super-admin`。
- [x] 4.4 确认 Makefile 示例使用 `ADMIN_BOOTSTRAP_PASSWORD`、`ADMIN_USERNAME` 和 `ADMIN_NICKNAME`，且命令行不展开或输出密码值。
- [x] 4.5 更新相关开发、测试或部署文档中的发布顺序为 `Atlas migration -> rbac seed -> bootstrap-super-admin -> 启动 HTTP 副本 -> 初始管理员强制改密`，移除旧命令示例。

## 5. E2E 与回归测试

- [x] 5.1 添加 CLI 回归测试，确认删除后的 `rbac create-super-admin`、`rbac assign-super-admin`、`--reset-password` 和旧 env 契约无法作为公开入口使用。
- [x] 5.2 添加 seed -> bootstrap -> 强制改密 -> 正常登录 -> 超级管理员授权 E2E，验证 bootstrap 用户首次登录只获得 password-change token，改密后可正常登录并具备超级管理员权限。
- [x] 5.3 添加后续授权流程验证，确认其他超级管理员只能通过在线用户角色绑定 API 授权，并触发现有 policy version 发布和缓存收敛路径。
- [x] 5.4 确认本次无 Ent schema 变化、无新增业务表和无 Atlas migration；如生成物意外变化，审查并回退非预期 drift。

## 6. 验证与收尾

- [x] 6.1 运行 bootstrap application 相关 Go package 测试。
- [x] 6.2 运行 role PostgreSQL store 集成测试和相关 RBAC CLI 测试。
- [x] 6.3 运行覆盖认证强制改密与超级管理员授权的 HTTP E2E。
- [x] 6.4 运行 `make user-service-architecture-lint`，确认 application、infrastructure、shared 和 common 边界未漂移。
- [x] 6.5 运行必要生成检查，确认不需要 `make user-service-generate`、`make user-service-migrate-diff` 或 OpenAPI 生成物更新，或在需要时执行并审查 diff。
- [x] 6.6 将本次预期代码、文档和 OpenSpec artifact 加到暂存区后运行 `make lint`。
- [x] 6.7 在暂存预期变更后运行 `make verify`，确保最终 drift 检查不会被未暂存的预期变更阻塞。
- [x] 6.8 根据实际验证结果更新本 `tasks.md` checkbox，并保留失败命令、失败原因和后续处理记录。
