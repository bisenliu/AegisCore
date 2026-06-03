## Why

当前用户表和 API 字段仍使用 `active`、`name`、`password` 等早期命名，无法准确表达用户生命周期状态、昵称语义和密码哈希存储语义，也缺少软删除字段来支持删除后保留历史数据。需要在一次受控 schema/API 变更中统一字段命名、状态枚举、请求校验和迁移契约，避免后续能力在旧字段上继续扩展。

## What Changes

- **BREAKING** 删除用户表、Ent 模型、DTO、接口响应和查询条件中的 `active` 字段，新增整型 `status` 字段。
- **BREAKING** 用户状态枚举改为 `100` 正常、`200` 冻结/停用、`300` 必须修改密码，并要求所有涉及 `status` 的请求参数通过 `common/validation` 已注册的 `enum`/`validateEnum` 规则校验。
- **BREAKING** 将持久化密码字段从 `password` 重命名为 `password_hash`，同步更新数据库字段、Ent schema、业务入参、repository 写入/读取和 Swagger/DTO 描述；外部响应仍不得返回密码哈希。
- **BREAKING** 将实际表示昵称的 `name` 字段统一重命名为 `nickname`，同步更新数据库字段、Ent schema、创建/查询/列表 DTO、过滤条件、序列化字段、Swagger 文档和业务映射。
- 新增 `deleted_at` 字段用于软删除，`NULL` 表示未删除；用户查询、列表、登录、唯一性检查等读取路径默认排除软删除记录。
- 修正 `status=300` 必须修改密码的登录行为：密码校验通过后不得签发普通会话 token，但必须返回仅可用于修改密码接口的受限认证凭据，避免用户因无法通过 token 校验而不能完成改密。
- 更新用户表索引、Atlas migration、Ent 生成代码、请求参数校验、接口文档和相关测试，确保数据模型与外部契约一致。
- 保留必要的历史兼容处理策略，仅用于已部署数据迁移和可控 API 过渡，不引入长期双字段运行时分支。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-profile-query`: 查询响应字段由 `name`/`active` 更新为 `nickname`/`status`，查询默认排除 `deleted_at` 非空用户且继续不暴露密码哈希。
- `user-profile-create`: 创建请求与持久化字段更新为 `nickname`、`password_hash`、`status`，默认状态为 `100`，状态请求校验使用共享 enum 规则。
- `user-list-query`: 列表响应、过滤白名单和 Swagger 文档从 `name`/`active` 更新为 `nickname`/`status`，默认排除软删除记录并对 `status` 使用共享 enum 校验。
- `user-session-control`: 登录和 token version 读取必须使用 `password_hash`；冻结/停用或软删除用户不得进入会话流程；必须改密用户只能获得受限改密凭据，不得获得普通会话权限。
- `request-validation`: 明确用户状态 DTO 必须通过共享 `enum`/`validateEnum` 校验，不得在 controller/service 中重复硬编码状态取值校验。
- `api-swagger-documentation`: 用户创建、查询、列表和认证相关文档必须反映 `nickname`、`status`、`password_hash` 输入语义、软删除过滤和响应字段约束。
- `database-schema-migrations`: 通过 Ent schema 和 Atlas migration 表达删除 `active`、新增 `status`、重命名 `password_hash`/`nickname`、新增 `deleted_at`、调整索引和迁移校验。

## Impact

- 代码范围：`user-services/ent/schema/user.go`、Ent 生成代码、`user-services/internal/controller`、`service`、`repository`、认证服务、请求/响应 DTO、Swagger 注解与生成产物、相关测试。
- 数据库范围：`users` 表字段、默认值、注释、索引、唯一性约束或过滤条件，以及 `user-services/migrations/` 和 `atlas.sum`。
- API 影响：用户创建、查询、列表和登录相关请求/响应字段发生破坏性命名变更；响应继续使用 `common/response.Envelope`。
- 认证影响：`status=300` 登录响应需要区分普通 token 与改密专用 token，认证中间件或改密接口必须只允许该凭据访问修改密码路径。
- 校验影响：`status` 请求参数必须实现枚举类型并通过 `validate:"enum"` 触发 `common/validation/validator.go` 注册的 `validateEnum`。
- 兼容性影响：旧数据库数据需要 migration 从 `active` 映射到 `status`，从 `name`/`password` 迁移为 `nickname`/`password_hash`；长期 API 字段以新命名为准。
