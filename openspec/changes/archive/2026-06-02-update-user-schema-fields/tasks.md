## 1. Schema And Generated Code

- [x] 1.1 更新 `user-services/ent/schema/user.go`，新增必填 `password` 字段。
- [x] 1.2 将 `created_at`、`updated_at` 改为 `int64` 毫秒级 Unix 时间戳字段，并保留创建默认值和更新默认值。
- [x] 1.3 为 `User` schema 中的 `id`、`name`、`email`、`password`、`active`、`created_at`、`updated_at` 增加数据库字段 comment。
- [x] 1.4 在 `user-services/` 模块运行 `go generate ./ent`，只通过生成流程更新 `user-services/ent/` 代码。

## 2. User Create And Query Implementation

- [x] 2.1 更新 `dto.CreateUserRequest`，增加必填 `password` 字段校验，并更新 Swagger 示例。
- [x] 2.2 更新 `dto.UserResponse`，将 `created_at`、`updated_at` 类型改为毫秒时间戳，并确保响应结构不包含 `password`。
- [x] 2.3 更新 service 创建逻辑，校验并传递 `password`，且不在日志、错误消息或响应中公开密码。
- [x] 2.4 更新 repository 创建输入和 Ent create 调用，写入 `password` 并保持 `name`、`email`、`active` 现有行为。
- [x] 2.5 更新查询映射逻辑，使 `created_at`、`updated_at` 按毫秒时间戳返回，且查询响应不包含 `password`。

## 3. Migration

- [x] 3.1 在 `user-services/` 运行 `./scripts/migrate-diff.sh update_user_schema_fields` 生成 Atlas SQL migration。
- [x] 3.2 审查生成的 SQL，确认包含非空 `password` 字段、时间字段毫秒时间戳变更和字段 comment 变更。
- [x] 3.3 如需手工调整 SQL 处理已有数据回填或类型转换，重新计算 migration directory checksum。
- [x] 3.4 在 `user-services/` 运行 `./scripts/migrate-validate.sh` 校验 migration 目录和 `atlas.sum`。

## 4. Tests And Documentation Output

- [x] 4.1 更新 service、repository、controller 和 HTTP bootstrap 相关测试夹具，补充 `password` 和毫秒时间戳断言。
- [x] 4.2 增加或更新创建用户缺少 `password` 的参数校验测试。
- [x] 4.3 增加或更新创建和查询响应不包含 `password` 的测试。
- [x] 4.4 更新 Swagger 注解或生成文档输出，使创建请求包含 `password`，响应时间字段为毫秒时间戳且不包含 `password`。
- [x] 4.5 在 `common/` 运行 `go test ./...`。
- [x] 4.6 在 `user-services/` 运行 `go test ./...`。

## 5. Final Verification

- [x] 5.1 运行 `gofmt -w` 格式化变更的 Go 文件。
- [x] 5.2 确认没有手写修改 `user-services/ent/` 生成代码以外的生成流程输出。
- [x] 5.3 确认 OpenSpec delta specs、design 和 tasks 覆盖密码字段、毫秒时间戳、字段 comment、migration 和响应不泄露密码。
