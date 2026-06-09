## MODIFIED Requirements

### Requirement: User list query uses repository abstraction with PostgreSQL implementation boundary
用户列表查询能力 SHALL 通过 `userapp.UserProfileStore` 抽象执行分页、过滤和排序相关数据访问，具体 Ent/PostgreSQL predicate 与查询实现 MUST 位于 `user-services/internal/features/user/infra/postgres` 包。实现 MUST NOT 新增根 `repository` 包承载列表查询输入类型或依赖具体实现包。

#### Scenario: List service remains decoupled from Ent query implementation
- **Given** 用户列表 service 需要查询用户列表
- **When** service 调用数据访问层
- **Then** service MUST 通过 `userapp.UserProfileStore.ListUsers` 提交列表查询输入
- **Then** service MUST NOT 直接引用 Ent predicate helper 或 `features/user/infra/postgres` 私有实现类型

#### Scenario: List query behavior remains compatible
- **Given** `features/user/infra/postgres` 承载 Ent/PostgreSQL 列表查询实现
- **When** 调用方请求用户列表 API
- **Then** 系统 MUST 继续按现有分页、过滤、排序和未软删除条件返回用户列表
- **Then** HTTP 响应信封、公开字段和错误语义 MUST 与迁移前保持一致

### Requirement: List service uses domain user collection
用户列表查询能力 SHALL 通过 `userapp.UserProfileStore` 获取用户领域实体集合和总数，并由 Service 层映射为分页响应 DTO。Service 层 MUST NOT 直接依赖 Ent 用户列表模型，列表 API 的过滤、分页、响应信封和公开字段 MUST 保持不变。

#### Scenario: List repository returns domain users
- **Given** Service 使用规范化后的分页和过滤条件调用用户 Repository
- **When** PostgreSQL repository 查询用户列表成功
- **Then** Repository MUST 将 Ent 用户模型集合转换为用户领域实体集合
- **Then** Repository MUST 返回用户领域实体集合和总数
- **Then** Service MUST NOT 遍历 `[]*ent.User` 构造列表响应

#### Scenario: List response remains compatible
- **Given** Service 获得用户领域实体集合和总数
- **When** Service 构造分页响应
- **Then** 响应 items MUST 继续只包含公开用户资料字段
- **Then** 响应 pagination MUST 继续使用现有页码、每页数量和总数语义
- **Then** 响应 MUST NOT 包含 `password_hash`、内部 `id` 或 `deleted_at`

#### Scenario: List service remains independent of Ent
- **Given** 用户列表查询 Service 编译
- **When** 检查 Service 层依赖
- **Then** `user-services/internal/features/user/app` MUST NOT 为用户列表查询导入 `github.com/aegiscore/user-services/ent`
- **Then** Ent 列表查询和 Ent 到 Domain 映射 MUST 位于 `user-services/internal/features/user/infra/postgres`
