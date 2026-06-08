## 1. 文件重命名

- [x] 1.1 将 `common/contract/response/failure.go` 重命名为 `common/contract/response/helpers.go`
- [x] 1.2 确认 `helpers.go` 仍使用 `package response`，且 `BadRequest`、`ValidationFailed`、`Unauthenticated`、`Forbidden`、`Conflict`、`NotFound` 等导出 helper 签名不变
- [x] 1.3 搜索仓库内 `failure.go` 文件名引用，仅同步更新文件名相关文档、测试或脚本引用

## 2. 契约兼容检查

- [x] 2.1 确认 `common/contract/response` 的导出 API、业务错误码常量、HTTP status 映射、公开 message 和 JSON 字段未因重命名改变
- [x] 2.2 确认 controller、中间件和服务代码继续通过既有 `github.com/aegiscore/common/contract/response` import path 调用响应 helper
- [x] 2.3 确认本变更不涉及 Ent schema、Atlas migration、配置、Redis/PostgreSQL 连接或 Fx lifecycle

## 3. 验证

- [x] 3.1 对重命名后的 Go 文件运行 `gofmt`
- [x] 3.2 在 `common/` 模块运行 `go test ./...`
- [x] 3.3 在 `user-services/` 模块运行 `go test ./...`，确认调用方响应 helper 使用不受影响
- [x] 3.4 运行 OpenSpec 状态或校验命令，确认 proposal、design、specs 和 tasks 已达到 apply readiness
