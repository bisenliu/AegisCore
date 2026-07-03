## ADDED Requirements

### Requirement: Role PostgreSQL store 持久化契约

系统 MUST 使用当前 Ent schema 和外部 UUID 字段实现 Role PostgreSQL store 的角色、用户角色绑定和角色权限绑定持久化；join 表内部外键只用于数据库关联，不得暴露为 role feature 的业务查询入口。store MUST 将当前领域错误稳定映射给 application 层，并在替换绑定失败时保持已有绑定不被部分破坏。

#### Scenario: 角色按外部 UUID 持久化和查询

- **WHEN** role infrastructure store 创建、查询、批量查询、列表、更新或启停角色
- **THEN** 系统 MUST 以 `roles.role_id` 作为稳定业务标识执行查询和排序
- **AND** 唯一约束冲突 MUST 映射为 `ErrRoleAlreadyExists`
- **AND** 未找到目标角色 MUST 映射为 `ErrRoleNotFound`

#### Scenario: 用户角色绑定使用当前用户和角色身份

- **WHEN** role infrastructure store 查询、添加、替换或移除用户角色绑定
- **THEN** 系统 MUST 通过用户外部 UUID 和角色外部 UUID 解析当前未软删除用户与角色
- **AND** 空绑定集合 MUST 返回空结果且不得创建兼容占位数据
- **AND** 重复或不存在的绑定引用 MUST 返回当前领域错误并保持已有绑定关系不被破坏

#### Scenario: 角色权限绑定复核当前启用权限

- **WHEN** role infrastructure store 添加或替换角色权限绑定
- **THEN** 系统 MUST 在写入前按权限外部 UUID 复核权限存在且处于启用状态
- **AND** 不存在或已停用权限 MUST 映射为当前权限或角色绑定领域错误
- **AND** 替换失败 MUST 回滚事务并保留替换前的角色权限绑定

#### Scenario: 不引入旧兼容查询和绑定语义

- **WHEN** Role PostgreSQL store 测试或实现覆盖角色与绑定持久化
- **THEN** 系统 MUST NOT 新增旧 internal ID 查询入口、旧 role code 字段、旧 binding 行为或兼容查询 helper
- **AND** 测试 MUST 以当前 Ent schema、当前外部 UUID 字段和当前领域错误为准
