## 1. 响应契约实现

- [x] 1.1 扩展 `common/response.Envelope` 或新增等价响应 helper，使失败响应可选输出 `errors` 字段且不影响普通失败响应。
- [x] 1.2 调整 `response.ValidationFailed` 或新增专用参数校验失败 helper，支持传入字段级错误明细并保持 HTTP 400、业务码 `10001`、顶层 message `请求参数验证失败`。

## 2. 校验错误归一化

- [x] 2.1 扩展 `common/validation.FieldError`，包含 `field`、`label`、`rule`、`message` 的 JSON 输出。
- [x] 2.2 在 validator tag 校验失败归一化逻辑中提取请求字段名、显示名、触发规则和中文错误消息。
- [x] 2.3 调整 `BindOrAbort` 的参数校验失败分支，把结构化字段错误明细传递给响应层。
- [x] 2.4 保持请求体为空、JSON 类型不匹配、URI/query/form 类型解析失败等 bad request 场景的现有响应行为。

## 3. 测试验证

- [x] 3.1 更新 `common/validation` 单元测试，覆盖 `label` tag、请求字段名、规则名和字段级错误消息。
- [x] 3.2 更新 `common/response` 单元测试或新增覆盖，验证普通失败响应不输出 `errors`，参数校验失败响应输出 `errors`。
- [x] 3.3 更新 `user-services` controller 测试，验证 API 参数校验失败响应包含用户指定的结构化错误数组。
- [x] 3.4 运行 `gofmt` 格式化被修改的 Go 文件。
- [x] 3.5 分别在 `common/` 和 `user-services/` 运行 `go test ./...`，确认共享包与用户服务测试通过。
