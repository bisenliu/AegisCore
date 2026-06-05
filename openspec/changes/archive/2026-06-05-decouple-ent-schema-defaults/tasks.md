## 1. Ent Schema Boundary

- [x] 1.1 在 `user-services/ent/schema/user/schema.go` 中移除 `internal/domain` import。
- [x] 1.2 在 Ent user schema 分类包内定义仅用于数据库 schema 声明的本地默认值常量，并保持 `status` 默认值为 `100`。
- [x] 1.3 确认 schema 本地默认值常量未被 controller、service 或 repository 用作业务状态规则来源。

## 2. Generated Code And Verification

- [x] 2.1 在 `user-services` 模块运行 `go generate ./ent`，不要手写 `user-services/ent/` 下的生成代码。
- [x] 2.2 审查 Ent 生成结果，确认 `status` 字段类型、默认值、注释和索引语义保持不变。
- [x] 2.3 审查 `user-services/migrations/` 和 `atlas.sum`，确认没有因默认值来源重构产生 SQL migration 或 checksum 变更。

## 3. Tests

- [x] 3.1 在 `user-services` 模块运行 `go test ./...`，确认用户服务编译和测试通过。
- [x] 3.2 在 `common` 模块运行 `go test ./...`，确认共享模块未受影响。
- [x] 3.3 检查用户查询、创建和认证相关接口没有 HTTP 路径、响应信封、错误码或公开 JSON 字段变化。
