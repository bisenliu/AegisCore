## 1. API Status Type Consolidation

- [x] 1.1 删除 `user-services/internal/features/user/api/request.go` 中重复的 `UserStatus` 类型、状态常量、`IsValid`、`AllowedValues`、`UnmarshalText` 和 `UnmarshalJSON`。
- [x] 1.2 在 `user-services/internal/features/user/api/request.go` 导入用户领域包，并将 `ListUsersRequest.Status` 与 `CreateUserRequest.Status` 改为 `*userdomain.UserStatus`。
- [x] 1.3 更新 `CreateUserRequest.SetDefaults`，使用 `userdomain.UserStatusNormal` 设置缺省状态。

## 2. Mapping And References

- [x] 2.1 简化或删除 `transport/http/controller.go` 中 `toCommandStatus` 的冗余类型转换，确保 command/query 直接接收领域状态类型。
- [x] 2.2 更新 `app/mapper.go` 和 `api/response.go` 中用户响应状态类型或映射，避免依赖已删除的 `userapi.UserStatus`。
- [x] 2.3 全量替换测试和业务代码中对 `userapi.UserStatus*` 的引用，改为 `userdomain.UserStatus*` 或领域状态类型。

## 3. Verification

- [x] 3.1 对修改的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `user-services/` 模块运行用户 feature 相关测试，至少覆盖 `internal/features/user/...`。
- [x] 3.3 确认本变更不涉及 Ent schema、Atlas migration、Redis key、运行时配置或 `go generate`。
