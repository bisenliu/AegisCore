## ADDED Requirements

### Requirement: Maintain user status default consistency

系统 SHALL 保持用户服务 Ent `User` schema 的 `status` 字段默认值与领域正常状态枚举一致。schema 默认值作为数据库持久化契约 MUST 继续为 `100`，并且实现 MUST 通过自动化测试防止该值与 `domain.UserStatusNormal` 静默漂移。仅新增一致性测试和注释时，系统 MUST NOT 生成新的 Atlas SQL migration 或修改既有 migration 历史。

#### Scenario: Schema default matches domain normal status
- **Given** `user-services/ent/schema/userschema` 定义 `User` schema 的 `status` 字段默认值
- **Given** `user-services/internal/domain` 定义 `domain.UserStatusNormal`
- **When** 用户服务测试运行
- **Then** 测试 MUST 验证 `status` 字段默认值等于 `domain.UserStatusNormal` 的数值
- **Then** 测试 MUST 在 schema 默认值或领域正常状态枚举发生单边修改时失败

#### Scenario: Comment documents persistent default contract
- **Given** Ent schema 使用 schema 本地默认值常量表达 `status` 字段默认值
- **When** 开发者阅读该默认值常量或其用途
- **Then** 注释 MUST 明确该值必须与 `domain.UserStatusNormal` 保持一致
- **Then** 生产 schema source MUST NOT 为复用领域常量而直接依赖 `user-services/internal/domain`

#### Scenario: Consistency guard does not change database schema
- **Given** 本次变更只新增一致性测试并补充默认值注释
- **When** 开发者审查 Ent schema 和迁移目录
- **Then** `status` 字段的数据库默认值 MUST 继续为 `100`
- **Then** 用户表字段类型、字段注释、索引和约束 MUST 保持不变
- **Then** `user-services/migrations/` MUST NOT 因本次变更新增 SQL migration
- **Then** 既有 `atlas.sum` MUST NOT 因无数据库语义变化而修改
