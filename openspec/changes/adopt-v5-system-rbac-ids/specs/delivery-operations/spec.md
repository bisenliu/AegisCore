## ADDED Requirements

### Requirement: 项目身份初始化与重命名 ID 边界

系统 MUST 区分从基础框架初始化新项目和已有项目重命名。新项目初始化 MAY 生成新的 RBAC 系统 ID namespace 和固化常量；已有项目重命名 MUST NOT 默认重算系统内置 RBAC、permission 或 bootstrap 用户 ID。

#### Scenario: 新项目初始化生成系统 ID
- **WHEN** AegisCore 作为基础框架复制为全新项目
- **THEN** 初始化流程 MAY 生成新的 `SystemIDNamespace`
- **AND** 初始化流程 MAY 基于固定 semantic name 列表生成 UUID v5 结果并写入 `user-service/internal/shared/rbacbaseline/ids.go`
- **AND** 初始化流程 MUST 只修改代码或文档工件，MUST NOT 连接数据库、修改已有数据库数据或执行 RBAC seed

#### Scenario: 已有项目重命名保持系统 ID
- **WHEN** 已落地项目修改项目展示名、服务名、module path、CLI 名、镜像名、部署资源名或观测 label
- **THEN** 重命名流程 MUST NOT 默认修改 `SystemIDNamespace`
- **AND** 重命名流程 MUST NOT 默认修改 `SuperAdminRoleID`、`BootstrapSuperAdminUserID` 或任一 baseline permission ID
- **AND** 重命名流程 MUST NOT 默认修改已有数据库中的角色、权限、用户或绑定 ID
- **AND** 如需重算系统 ID，MUST 作为单独高风险数据迁移 change 处理，不得混入普通重命名流程

#### Scenario: 重命名脚本文档约束
- **WHEN** 仓库提供项目初始化脚本或项目重命名脚本
- **THEN** 文档 MUST 明确初始化脚本只适用于新项目创建
- **AND** 文档 MUST 明确重命名脚本默认不重算系统 ID
- **AND** 脚本或 README MUST NOT 宣称已有项目改名会自动迁移 RBAC 系统 ID
