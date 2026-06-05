## 1. Schema Contract Documentation

- [x] 1.1 在 `user-services/ent/schema/userschema/schema.go` 的 `defaultUserStatus` 附近补充注释，明确该持久化默认值必须与 `domain.UserStatusNormal` 保持一致。
- [x] 1.2 确认 `status` 字段类型、默认值数字、字段注释、索引和约束未发生数据库语义变化。

## 2. Consistency Test

- [x] 2.1 在 `user-services/ent/schema/userschema` 附近新增或更新测试，读取 `User` schema 的 `status` 字段默认值。
- [x] 2.2 在测试中断言 `status` 默认值等于 `int64(domain.UserStatusNormal)`，并保证单边修改 schema 默认值或领域枚举时测试失败。
- [x] 2.3 保持生产 schema source 不直接导入 `user-services/internal/domain`，仅测试代码引用领域枚举完成一致性断言。

## 3. Verification

- [x] 3.1 对修改的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `user-services/` 模块运行 `go test ./...`。
- [x] 3.3 确认未修改 `user-services/ent/` 生成代码、`user-services/migrations/` 或 `user-services/migrations/atlas.sum`。
