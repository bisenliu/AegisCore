## 1. 代码迁移

- [x] 1.1 新增 `user-services/internal/errmsg/messages.go`，迁移现有 `Msg*` 中文错误消息常量并保持常量名称和值不变。
- [x] 1.2 更新 user-services 内所有 `internal/apperror` import 为 `internal/errmsg`。
- [x] 1.3 更新所有 `apperror.Msg*` 引用为 `errmsg.Msg*`。
- [x] 1.4 删除 `user-services/internal/apperror/messages.go` 和不再使用的旧包目录。

## 2. 验证

- [x] 2.1 全仓库搜索 `internal/apperror` 和 `apperror.Msg`，确认旧包引用已清除。
- [x] 2.2 运行 `gofmt` 格式化受影响的 Go 文件。
- [x] 2.3 在 `user-services` 模块运行 `go test ./...`，确认迁移后编译与测试通过。
- [x] 2.4 如有相关响应测试，确认错误响应的 HTTP 状态码、业务错误码和 `message` 文本未变化。
