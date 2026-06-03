## 1. 公共密码能力

- [x] 1.1 在 `common` 模块新增 `common/password` 包，封装 Argon2id 默认参数、随机盐生成、hash 编码和解析逻辑。
- [x] 1.2 实现统一公共方法 `Hash(plain string) (string, error)`，确保输出包含算法、版本、内存、迭代次数、并行度、salt 和派生值。
- [x] 1.3 实现统一公共方法 `Verify(plain, encodedHash string) (bool, error)`，使用编码参数重新计算 hash 并执行常量时间比较。
- [x] 1.4 为 `common/password` 增加单元测试，覆盖成功哈希、正确密码校验、错误密码校验、格式非法 hash、空输入和不泄露敏感信息。
- [x] 1.5 更新 `common/go.mod` 依赖，确保 `golang.org/x/crypto/argon2` 可用且 `go mod tidy` 后依赖一致。

## 2. 用户模型与迁移

- [x] 2.1 更新 `user-services/ent/schema/user.go`，删除 `email` 字段，新增非空唯一 `username` 字段和非空唯一不可变 `user_id` UUID 字段，并补齐字段注释。
- [x] 2.2 移除业务代码对 Ent `Email` 字段、`SetEmail`、`EmailEQ` 和 email validator 生成符号的直接引用，改为等待生成后的 `Username`/`UsernameEQ` 符号。
- [x] 2.3 确认 `password` 字段语义为 Argon2id hash 存储，保持不映射到对外响应。
- [x] 2.4 在 `user-services` 模块运行 `go generate ./ent`，只通过生成流程更新 `user-services/ent/` 代码。
- [x] 2.5 运行 `./scripts/migrate-diff.sh refactor_user_identity_auth` 生成 Atlas SQL migration。
- [x] 2.6 人工审查 migration SQL，确认新增 `user_id`、新增 `username`、删除 `email` 列、删除 email 唯一约束、唯一约束变更、字段注释和历史数据回填策略正确。
- [x] 2.7 如手工调整 migration SQL，运行 Atlas hash/checksum 更新流程并同步 `user-services/migrations/atlas.sum`。
- [x] 2.8 运行 `./scripts/migrate-validate.sh`，确认 migration 目录校验通过。

## 3. 创建用户接口

- [x] 3.1 更新创建用户请求 DTO，将 `email` 替换为 `username`，保留 `name`、`password`、`active` 校验并移除邮箱格式校验。
- [x] 3.2 更新创建用户响应 DTO，返回 `user_id`、`name`、`username`、`active`、`created_at`、`updated_at`，不返回 `id`、`email`、`password` 或密码 hash。
- [x] 3.3 在 service 层创建用户时生成 UUIDv7 `user_id`，并调用 `common/password.Hash` 生成 Argon2id 密码 hash。
- [x] 3.4 更新 repository 创建逻辑，按 `username` 检查唯一性并持久化 `user_id`、`username`、`name`、`active`、`password` hash。
- [x] 3.5 删除 `UserRepository` 接口和实现中的 `ExistsByEmail`，新增或改用 `ExistsByUsername`，并更新所有 stub/mock/test helper。
- [x] 3.6 更新唯一性冲突映射，确保 `username` 或 `user_id` 数据库唯一冲突返回统一 HTTP 409 响应。
- [x] 3.7 更新创建用户 controller/service/repository 测试，覆盖成功创建、缺少字段、用户名重复、密码 hash 失败、唯一约束冲突和响应字段不泄露。

## 4. 查询用户接口与响应结构

- [x] 4.1 将用户查询路由从 `GET /api/v1/users/:id` 调整为 `GET /api/v1/users/:user_id`。
- [x] 4.2 更新查询 controller 参数解析，校验 `user_id` 为合法 UUID 字符串，并继续使用共享校验器中文错误消息。
- [x] 4.3 更新 service/repository 查询逻辑，按 `user_id` 查询用户并保持 not found、internal error 映射不泄露数据库细节。
- [x] 4.4 更新用户资料响应 mapper，确保查询响应只包含 `user_id`、`name`、`username`、`active`、`created_at`、`updated_at`。
- [x] 4.5 更新查询用户测试，覆盖成功查询、未认证、非法 UUID、用户不存在、数据库内部错误和响应不包含 `id`、`email`、`password`。

## 5. 用户列表接口

- [x] 5.1 更新用户列表请求 DTO，将 `email` 过滤字段替换为 `username`，并保持 `name`、`active`、分页字段行为。
- [x] 5.2 更新用户列表 service 清洗逻辑，移除邮箱小写规范化，改为清洗 `username`。
- [x] 5.3 更新用户列表 repository 输入结构，将 `Email` 字段替换为 `Username`，Ent 查询谓词从 `user.EmailEQ` 改为 username 字段谓词。
- [x] 5.4 更新用户列表响应 mapper，确保列表 item 只包含 `user_id`、`name`、`username`、`active`、`created_at`、`updated_at`。
- [x] 5.5 更新用户列表 controller/service/repository 测试，覆盖 username 过滤、email query 被忽略或不支持、分页、active 过滤和响应不包含 `id`、`email`、`password`。

## 6. 登录、JWT 与会话控制

- [x] 6.1 更新登录请求 DTO 和 Swagger，将登录凭据从 `email` 改为 `username`，并拒绝仅提交邮箱的登录请求。
- [x] 6.2 删除 auth repository/service 中的 `GetByEmail` 登录路径，新增或改用 `GetByUsername` 并读取 `user_id`、`password` hash 和 `token_version`。
- [x] 6.3 将认证日志字段从 `email` 改为 `username`，并确认不会记录密码明文、完整 hash、salt 或 hash 参数。
- [x] 6.4 更新登录密码校验调用点，统一调用 `common/password.Verify`，并将用户不存在、密码错误和 hash 格式非法映射为统一凭据无效响应。
- [x] 6.5 更新 JWT 签发逻辑，Access Token 和 Refresh Token claims 的 `user_id` 使用 UUID 外部用户标识，不使用内部数字主键。
- [x] 6.6 更新 JWT 解析和认证中间件，校验 `user_id` 为非空 UUID 字符串，并把该值写入 Gin context 和 Go context。
- [x] 6.7 更新 token version 查询、Redis session 记录和用户活跃会话索引，按外部 `user_id` 回源 PostgreSQL 或组织缓存 key。
- [x] 6.8 更新退出当前设备、退出全部设备和刷新 token 流程中所有用户身份调用点，确保不再依赖内部 `id`。
- [x] 6.9 更新认证与会话测试，覆盖用户名登录成功、邮箱登录被拒、密码错误、hash 格式非法、claims 使用 UUID、token version 回源和 session key 行为。

## 7. 文档、Swagger 与响应契约

- [x] 7.1 更新 Swagger 注解和生成文档，创建用户、查询用户、列表用户、登录接口字段均使用 `username` 和 `user_id`。
- [x] 7.2 移除 Swagger 生成文件和示例中的用户 `email` 字段、`email` query 参数和邮箱登录示例。
- [x] 7.3 更新 API 响应示例，保持 `common/response.Envelope` 外层结构不变，用户资料 `data` 不再展示 `id` 或 `email`。
- [x] 7.4 更新 `docs/ARCHITECTURE.md` 的用户数据模型、HTTP flow 和 API 路由说明。
- [x] 7.5 更新 `docs/opsx/CAPABILITY_MAP.md` 中相关 capability 描述或主要代码位置，确保反映用户名登录、列表 username 过滤和外部 `user_id`。

## 8. 验证与收尾

- [x] 8.1 在 `common` 模块运行 `go test ./...`，修复公共密码能力相关失败。
- [x] 8.2 在 `user-services` 模块运行 `go test ./...`，修复用户模型、API、列表、认证和会话相关失败。
- [x] 8.3 运行 `gofmt -w` 格式化所有修改的 Go 文件。
- [x] 8.4 运行 `go mod tidy` 并检查 `common/go.mod`、`common/go.sum`、`user-services/go.mod`、`user-services/go.sum` 是否只包含必要依赖变更。
- [x] 8.5 使用代码搜索确认 `user-services/internal` 和 Swagger 源注解中不存在 `Email` DTO 字段、`ExistsByEmail`、`GetByEmail`、`EmailEQ`、`email` 日志字段或用户 email 响应字段。
- [x] 8.6 复查所有用户资料响应、认证日志和错误消息，确认不暴露内部 `id`、`email`、明文密码、完整密码 hash、salt 或底层数据库细节。
- [x] 8.7 运行 `openspec status --change refactor-user-identity-auth`，确认 proposal、design、specs 和 tasks 均完成且可进入 apply。
