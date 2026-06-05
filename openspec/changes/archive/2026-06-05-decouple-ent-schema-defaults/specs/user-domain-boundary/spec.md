## ADDED Requirements

### Requirement: Ent schema defaults remain independent from domain layer

系统 SHALL 将 Ent schema 的数据库默认值声明限制在 schema 层或更低层的持久化契约内。Ent schema MUST NOT 为声明数据库字段默认值导入 `user-services/internal/domain`；业务状态规则 MUST 继续由领域模型和 repository 映射边界承载。

#### Scenario: User status default is declared without domain import
- **Given** `User` Ent schema 需要为 `status` 字段声明数据库默认值
- **When** Ent codegen 或 Atlas schema source 编译 `user-services/ent/schema/`
- **Then** `User` Ent schema MUST NOT 导入 `user-services/internal/domain`
- **Then** `status` 字段默认值 MUST 保持为数据库契约值 `100`
- **Then** Service 层状态判断 MUST 继续通过 `domain.UserStatus` 或用户领域实体表达

#### Scenario: Repository remains the domain mapping boundary
- **Given** PostgreSQL repository 从 Ent 用户模型读取 `status` 数值
- **When** Repository 返回用户数据给 Service
- **Then** Repository MUST 将 Ent `status` 数值转换为 `domain.UserStatus`
- **Then** Ent schema 本地默认值常量 MUST NOT 成为 Service 或 Controller 的业务状态规则来源
