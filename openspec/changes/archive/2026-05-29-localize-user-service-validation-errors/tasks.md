## 1. 错误路径确认

- [x] 1.1 复查 `user-services/internal/controller/user_controller.go` 的参数绑定失败路径，确认 `ctl.validator.Bind` 返回的错误当前被硬编码英文消息覆盖。
- [x] 1.2 复查 `common/validation` 对 URI 绑定错误、非正数校验错误和公开错误消息的归一化行为，确认是否需要补充中文化处理。

## 2. 实现

- [x] 2.1 修改用户查询 controller，在 `validation.URIBinder` 失败时使用共享校验器返回的公开错误消息输出 `response.BadRequest`。
- [x] 2.2 移除用户查询 controller 中的 `fmt.Println` 调试输出以及不再需要的 `fmt` import。
- [x] 2.3 如 URI 类型转换错误仍会暴露英文底层错误，在 `common/validation` 中补充最小归一化逻辑，使非数字 ID 返回中文公开错误消息。

## 3. 测试

- [x] 3.1 更新 `user-services/internal/controller/user_controller_test.go`，分别断言非数字 ID 和非正数 ID 返回 HTTP 400、`BAD_REQUEST`，且 `message` 不再是 `invalid user id`。
- [x] 3.2 如修改 `common/validation`，补充或更新 `common/validation/validation_test.go` 覆盖 URI 绑定类型错误的中文消息。
- [x] 3.3 确认成功查询、用户不存在和内部错误测试仍保持既有响应语义。

## 4. 验证

- [x] 4.1 对修改过的 Go 文件运行 `gofmt`。
- [x] 4.2 在 `common/` 运行 `go test ./...`。
- [x] 4.3 在 `user-services/` 运行 `go test ./...`。
- [x] 4.4 复核本变更不涉及 Ent 生成代码、Atlas migration、配置或数据库结构变更。
