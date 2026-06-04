## ADDED Requirements

### Requirement: List service uses domain user collection
用户列表查询能力 SHALL 通过根 `repository.UserRepository` 获取用户领域实体集合和总数，并由 Service 层映射为分页响应 DTO。Service 层 MUST NOT 直接依赖 Ent 用户列表模型，列表 API 的过滤、分页、响应信封和公开字段 MUST 保持不变。

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
- **Then** `user-services/internal/service` MUST NOT 为用户列表查询导入 `github.com/aegiscore/user-services/ent`
- **Then** Ent 列表查询和 Ent 到 Domain 映射 MUST 位于 `user-services/internal/repository/postgres`
