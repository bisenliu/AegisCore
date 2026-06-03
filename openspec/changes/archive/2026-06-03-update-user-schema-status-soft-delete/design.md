## Context

用户服务当前围绕 Ent `User` schema、controller/service/repository 分层、Swagger 注解和 Atlas migration 维护用户资料创建、查询、列表和会话能力。现有字段 `active`、`name`、`password` 已经出现在数据库、Ent 生成代码、DTO、查询过滤、Swagger 文档和测试中，字段语义不够明确，也无法表达冻结/停用、必须修改密码和软删除等用户生命周期状态。

本变更是跨能力的数据模型和 API 契约变更，必须同时处理数据库迁移、Ent 代码生成、请求校验、业务查询条件、接口文档和测试。所有 HTTP 响应仍通过 `common/response.Envelope` 输出，controller 只负责请求绑定和响应，service 负责编排和业务判断，repository 负责 Ent 查询和持久化。

## Goals / Non-Goals

**Goals:**

- 将用户核心字段统一为 `nickname`、`password_hash`、`status`、`deleted_at`，并删除 `name`、`password`、`active` 的长期运行时依赖。
- 使用用户服务内的状态枚举类型表达 `100` 正常、`200` 冻结/停用、`300` 必须修改密码，并让所有 status 请求 DTO 通过 `validate:"enum"` 使用共享 `validateEnum` 校验。
- 让用户查询、列表、创建唯一性检查和认证读取默认只处理 `deleted_at IS NULL` 的用户。
- 允许 `status=300` 用户在密码校验通过后获得仅限修改密码的受限认证凭据，避免无法通过 token 校验而不能完成强制改密。
- 通过 Ent schema、生成代码和 Atlas migration 表达数据库变更，不在运行时自动建表或改表。
- 更新 Swagger/OpenAPI、DTO、序列化字段、索引、测试和兼容迁移说明，使外部契约与实现一致。

**Non-Goals:**

- 不新增用户删除 API、用户冻结 API、修改密码 API 或后台管理能力。
- 不新增长期同时支持旧字段和新字段的双写/双读逻辑。
- 不把用户业务状态枚举迁移到 `common`，因为该枚举属于用户域专属业务语义。
- 不改变响应信封、Redis/PostgreSQL 命名实例或 HTTP 路由基础路径。

## Decisions

1. 用户状态枚举保留在 `user-services` 用户域包内。

   状态 `100`、`200`、`300` 是用户业务语义，不属于跨服务公共枚举。实现时定义具名整型类型和常量，并实现 `IsValid() bool`，请求 DTO 使用该类型和 `validate:"enum"`。替代方案是把状态放入 `common`，但这会把用户域语义泄漏到共享模块，违反 `shared-enum-contracts` 对服务业务常量的边界要求。

2. `status` 替代 `active`，默认值为 `100`。

   `active` 只能表达布尔启停，无法表示必须修改密码。创建用户未显式传入状态时由 service/DTO 默认和 Ent schema 默认共同保证为正常状态。旧数据 migration 将 `active=true` 映射为 `100`，`active=false` 映射为 `200`。替代方案是保留 `active` 并新增单独状态字段，但会造成状态来源冲突和查询条件分裂。

3. `password` 重命名为 `password_hash`，对外响应继续不返回。

   数据库字段和内部模型使用 `password_hash` 明确存储的是哈希值。创建/登录等输入语义应区分明文密码请求字段和持久化密码哈希字段：外部创建请求可以继续接收用户提交的密码值，但 service/repository 入参和 Ent 字段必须是 `password_hash`。Swagger 必须避免把 `password_hash` 描述为普通响应字段。替代方案是外部请求也强制使用 `password_hash`，但这会误导客户端提交哈希且削弱服务端负责哈希处理的边界。

4. `name` 统一重命名为 `nickname`。

   如果该字段实际表示展示昵称，则数据库、Ent schema、DTO、JSON/form/query tag、Swagger 和过滤条件都使用 `nickname`。旧数据 migration 将 `name` 转入 `nickname`。替代方案是保留 `name` 作为 API 字段别名，但本变更目标是消除歧义，长期兼容别名会扩大测试和文档成本。

5. 软删除通过 nullable `deleted_at` 毫秒时间戳表达。

   `deleted_at IS NULL` 表示未删除，非空表示软删除时间。查询用户、列表用户、创建前唯一性检查、登录和 token version 读取均默认排除软删除记录。当前不新增删除接口，但 schema 和查询条件先建立软删除基础。替代方案是布尔 `deleted` 字段，但无法记录删除时间且不利于审计。

6. 数据库变更通过 Ent schema、`go generate ./ent` 和 Atlas migration 完成。

   修改 `user-services/ent/schema/user.go` 后生成 Ent 代码，再通过用户服务 Atlas 脚本生成 migration。migration 应包含字段重命名或数据复制、默认值回填、非空约束、字段 comment、索引调整和 `atlas.sum` 更新。不得手写 `user-services/ent/` 下生成代码，也不得在 HTTP runtime 中调用 `client.Schema.Create(ctx)`。

7. 索引和唯一性以未删除用户为默认业务集合。

   邮箱唯一性和列表过滤应以未删除记录为默认范围。若 PostgreSQL partial unique index 适配当前迁移工具链可行，优先让 `email` 唯一约束仅作用于 `deleted_at IS NULL` 的记录；否则先保持全表唯一并在设计/任务中记录不能复用软删除邮箱的限制。无论选择哪种索引，repository 查询必须显式加未删除 predicate。

8. `status=300` 使用受限改密认证流程，而不是直接拒绝登录。

   登录时仍必须先校验用户名、未软删除状态和密码哈希。`status=100` 签发普通 Access Token、Refresh Token 并创建 Redis 会话；`status=200` 继续拒绝登录；`status=300` 在密码校验通过后签发短 TTL 的改密专用 token，token 必须能被修改密码接口识别并通过认证，但不得创建普通 Redis Refresh Token 会话，也不得访问普通受保护 API。推荐做法是在 JWT 中增加明确的 subject/scope，例如 `password_change`，并在认证中间件或修改密码路由层校验该 subject/scope；普通 API 只接受普通 access token。替代方案是在 status=300 时签发普通 access token 再由每个业务接口检查用户状态，但这会把限制分散到多个 controller/service，容易漏拦截。

## Risks / Trade-offs

- [破坏性 API 字段变更导致旧客户端失败] → Swagger、DTO、测试和变更说明统一使用新字段，并避免长期双字段兼容；如需灰度，由调用方在发布计划中安排客户端同步升级。
- [迁移中字段 rename 与数据回填顺序不当导致数据丢失] → 生成 SQL 后人工审查，优先使用可保留数据的 rename 或 add-copy-drop 流程，并重新计算 `atlas.sum`。
- [Ent 生成代码和手写代码不同步] → schema 修改后在 `user-services` 模块运行 `go generate ./ent`，再运行相关 Go 测试。
- [状态校验绕过共享 enum 规则] → 所有含 `status` 的请求 DTO 使用实现 `IsValid()` 的枚举类型和 `validate:"enum"`，测试覆盖非法状态值。
- [软删除过滤遗漏导致已删除用户可见或可登录] → 在 repository 层集中添加 `deleted_at IS NULL` predicate，并为查询、列表、登录、创建唯一性检查补测试。
- [必须改密用户无法调用修改密码接口] → `status=300` 登录成功后返回受限改密凭据，并只允许该凭据访问修改密码接口。
- [受限改密凭据误用于普通 API] → token subject/scope 与普通 access token 区分，普通认证路径必须拒绝改密 token，修改密码接口必须显式要求改密 token 或普通已认证用户。
- [部分唯一索引与 Ent/Atlas 表达能力不匹配] → 迁移 SQL 可人工调整并通过 Atlas checksum 管理；若暂不启用 partial unique index，则明确保留全表邮箱唯一约束。

## Migration Plan

1. 修改 Ent `User` schema：删除 `active`，新增 `status`、`deleted_at`，重命名 `name` 为 `nickname`、`password` 为 `password_hash`，补齐字段 comment、默认值和索引定义。
2. 更新手写代码：用户状态枚举、DTO tag、controller/service/repository、认证服务、受限改密 token 处理、mapper、Swagger 注解和测试。
3. 在 `user-services` 模块运行 `go generate ./ent`，确保生成代码来自 schema。
4. 运行 Atlas migration 生成脚本，审查 SQL 是否正确迁移旧数据：`name` 到 `nickname`、`password` 到 `password_hash`、`active` 到 `status`、`deleted_at` nullable 默认空。
5. 如需人工调整索引或数据迁移 SQL，修改后重新计算并校验 `user-services/migrations/atlas.sum`。
6. 运行 `go test ./...` 于 `common/` 和 `user-services/`，并重新生成 Swagger 文档。
7. 部署前先在目标环境 apply 已提交 migration，再启动用户服务 runtime。

Rollback 策略：该变更包含破坏性字段重命名和数据模型收敛，推荐通过数据库备份和应用版本回滚配合处理；一旦删除旧字段并发布新 API，自动无损回滚到旧 API 不作为目标。

## Open Questions

- 旧客户端是否需要短期 API 兼容窗口由发布计划决定；本变更规格默认长期契约只保留新字段。
- 邮箱唯一索引是否改为 `deleted_at IS NULL` partial unique index 取决于当前 Atlas/Ent 生成结果和人工 SQL 审查结论。
