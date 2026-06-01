## 1. 代码现代化

- [x] 1.1 搜索仓库中手写 Go 代码里的 `interface{}` 空接口用法，排除 `user-services/ent/` 等生成代码。
- [x] 1.2 将确认属于空接口类型的 `interface{}` 替换为 `any`，不改变函数签名语义和调用行为。
- [x] 1.3 对修改过的 Go 文件运行 `gofmt`。

## 2. 校验失败日志

- [x] 2.1 修改 `common/validation/validation.go` 的 `BindOrAbort`，将无效请求日志从 `logger.Warn` 调整为 `logger.Error`。
- [x] 2.2 在 `BindOrAbort` 中复用 `validationDetails(err)` 获取字段级明细，并在明细非空时追加结构化日志字段 `errors`。
- [x] 2.3 保持 `ValidationFailedWithErrors`、`BadRequest`、HTTP 状态码、业务错误码和响应 message 行为不变。

## 3. 测试与验证

- [x] 3.1 增加或更新 `common/validation` 相关测试，覆盖 `BindOrAbort` 校验失败时记录 error 级别日志。
- [x] 3.2 增加或更新测试，覆盖 validator tag 校验失败时日志包含 `errors` 字段，非字段级错误不要求非空 `errors` 字段。
- [x] 3.3 在 `common/` 目录运行 `go test ./...`。
- [x] 3.4 在 `user-services/` 目录运行 `go test ./...`，确认跨模块编译和测试不受影响。
