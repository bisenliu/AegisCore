## 1. 消息包迁移

- [x] 1.1 将 `user-services/internal/errmsg/messages.go` 迁移到 `user-services/internal/messages/messages.go`，并将 package 声明改为 `messages`
- [x] 1.2 将常量从 `MsgInvalidUsername`、`MsgInvalidUserID`、`MsgInvalidPassword`、`MsgInvalidCredentials`、`MsgUserAlreadyExists`、`MsgUserNotFound`、`MsgMissingSession` 重命名为无 `Msg` 前缀的语义名称
- [x] 1.3 删除或停止使用旧的 `user-services/internal/errmsg` 包，确保不保留行为分叉的旧消息来源

## 2. 文案优化

- [x] 2.1 建立旧文案到新文案的映射，确认每条文案只优化表达、不改变原始业务语义
- [x] 2.2 将空用户名、用户 ID 格式错误、空密码、凭证错误、用户已存在、用户不存在和会话无效文案更新为统一、专业、适合直接展示给最终用户的中文表达
- [x] 2.3 检查优化后的文案不得泄露内部数据库、密码、token 签名、底层依赖或调试细节

## 3. 引用同步

- [x] 3.1 更新 user-services 中所有 `internal/errmsg` import 为 `internal/messages`
- [x] 3.2 更新所有 `errmsg.Msg*` 或 `Msg*` 常量引用为 `messages.*` 新名称
- [x] 3.3 使用仓库搜索确认 `user-services/internal/errmsg`、`errmsg.` 和旧 `Msg*` 消息常量名不再存在于业务代码中

## 4. 验证

- [x] 4.1 对修改过的 Go 文件运行 `gofmt`
- [x] 4.2 在 `user-services/` 模块运行 `go test ./...`，确认包迁移和文案更新后测试通过
- [x] 4.3 如测试包含精确 message 断言，同步更新预期文案，并确认 HTTP status、业务错误码和响应信封字段未改变
