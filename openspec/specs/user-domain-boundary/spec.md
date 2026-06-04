# user-domain-boundary

## Purpose

用户领域边界能力定义 Service、Repository、Ent 持久化模型与领域实体之间的依赖边界，确保用户状态规则和对外业务数据由领域模型承载，持久化实现细节不泄漏到上层。

## Requirements
### Requirement: User domain model owns user state rules
系统 SHALL 在 `user-services/internal/domain` 中提供用户领域实体，用于表达用户对外身份、基础资料、认证凭据摘要、状态、token version 和公开时间戳。用户状态相关业务判断 MUST 通过用户领域实体或用户状态枚举方法表达，Service 层 MUST NOT 直接依赖 Ent 用户模型字段实现用户状态规则。

#### Scenario: Domain user represents service user data needs
- **Given** Service 层需要处理用户查询、创建响应、登录、改密或 token version 相关流程
- **When** Service 从用户 Repository 获取用户数据
- **Then** Repository MUST 返回用户领域实体
- **Then** 用户领域实体 MUST 包含 Service 当前业务所需的 `user_id`、`nickname`、`username`、`password_hash`、`status`、`token_version`、`created_at` 和 `updated_at`
- **Then** Service MUST NOT 为读取这些字段导入 Ent 用户模型

#### Scenario: User state rules are centralized
- **Given** 登录或改密流程需要判断用户是否正常、禁用或必须修改密码
- **When** Service 执行用户状态判断
- **Then** Service MUST 使用用户领域实体或 `domain.UserStatus` 提供的方法表达状态规则
- **Then** Service MUST NOT 通过散落的 Ent 字段类型转换重复实现相同状态规则

### Requirement: Persistence models remain repository implementation details
系统 SHALL 将 Ent 用户模型限制在 PostgreSQL repository 实现边界内。根 `repository.UserRepository` 抽象 MUST 面向领域模型和领域错误，不得向 Service、Controller 或 DTO 层暴露 Ent 生成类型。

#### Scenario: Postgres repository maps Ent user to domain user
- **Given** PostgreSQL repository 使用 Ent client 查询或创建用户记录
- **When** Repository 方法需要返回用户数据给 Service
- **Then** PostgreSQL repository MUST 将 Ent 用户模型转换为用户领域实体
- **Then** Ent 用户模型 MUST NOT 作为根 `repository.UserRepository` 方法返回类型

#### Scenario: Repository preserves domain errors
- **Given** Ent 查询返回 not found 或唯一约束冲突
- **When** PostgreSQL repository 处理该错误
- **Then** Repository MUST 继续返回 `domain.ErrUserNotFound` 或 `domain.ErrUserAlreadyExists`
- **Then** Repository MUST NOT 构造 `common/response` 应用错误

#### Scenario: Internal refactor preserves external contracts
- **Given** 用户领域边界调整完成
- **When** 调用方访问现有用户资料或认证会话 API
- **Then** HTTP 路径、响应信封、公开 JSON 字段和错误码 MUST 保持兼容
- **Then** 数据库 schema、Atlas migration、Ent 生成代码和 Redis key 格式 MUST 保持不变
