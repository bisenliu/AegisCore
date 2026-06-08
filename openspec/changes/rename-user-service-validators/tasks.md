## 1. 目录与包迁移

- [x] 1.1 创建 `user-services/internal/validators/` 并将现有 `user-services/internal/validation/` 逻辑迁移到新包名 `validators`
- [x] 1.2 将用户资料相关函数迁移到 `user-services/internal/validators/user.go`，包括 `NormalizeCreateUser`、`NormalizeListUsers` 和 `ParseUserID`
- [x] 1.3 将认证相关函数迁移到 `user-services/internal/validators/auth.go`，包括 `NormalizeLogin`、`NormalizeChangePassword` 和 `NormalizeRefresh`
- [x] 1.4 删除旧的 `user-services/internal/validation/` 目录，确保代码中不再引用旧 import path

## 2. 测试迁移

- [x] 2.1 将用户资料相关测试迁移到 `user-services/internal/validators/user_test.go`
- [x] 2.2 将认证相关测试迁移到 `user-services/internal/validators/auth_test.go`
- [x] 2.3 更新测试 package 为 `validators`，确保测试继续覆盖 trim、lowercase、分页默认值、UUID parse、token 清洗和错误映射

## 3. 调用方更新

- [x] 3.1 更新 `user-services/internal/controller/user_controller.go`，使用 `github.com/aegiscore/user-services/internal/validators` 并调用 `validators.NormalizeCreateUser`、`validators.NormalizeListUsers`、`validators.ParseUserID`
- [x] 3.2 更新 `user-services/internal/controller/auth_controller.go`，使用 `github.com/aegiscore/user-services/internal/validators` 并调用 `validators.NormalizeLogin`、`validators.NormalizeChangePassword`、`validators.NormalizeRefresh`
- [x] 3.3 检查 user-services 其他 Go 文件，确保没有遗留 `user-services/internal/validation` 引用

## 4. 规格与文档同步

- [x] 4.1 更新 `openspec/specs/user-service-validation-boundary/spec.md` 中服务内校验边界路径和包名引用
- [x] 4.2 更新 `openspec/specs/request-validation/spec.md` 中服务特定校验边界引用，保持 `common/validation` 共享核心定位不变
- [x] 4.3 更新 `docs/opsx/CAPABILITY_MAP.md` 或其他文档中涉及 `user-services/internal/validation` 的路径引用

## 5. 验证

- [x] 5.1 运行 `gofmt` 格式化迁移后的 Go 文件
- [x] 5.2 在 `user-services/` 模块运行 `go test ./...`
- [x] 5.3 在 `common/` 模块运行 `go test ./...`，确认共享校验核心行为未受影响
- [x] 5.4 运行 OpenSpec 校验或状态检查，确认 change artifacts 和 spec delta 可被识别
