## 1. 共享校验实现

- [x] 1.1 在 `common/validation` 中增加可选 enum 允许值接口，保持现有 `Enum` 兼容。
- [x] 1.2 更新 enum 翻译注册和失败文案，支持 `{字段名}取值不合法，允许值为：{值1}、{值2}、{值3}`。
- [x] 1.3 处理 nil 指针、未实现 enum 接口和未提供允许值列表的安全降级文案。

## 2. 业务枚举接入

- [x] 2.1 为用户状态枚举提供固定顺序的允许值列表。
- [x] 2.2 确认 controller/service 不新增重复的状态取值校验逻辑。

## 3. 测试与验证

- [x] 3.1 补充 common validation 单元测试，覆盖 enum 允许值消息和降级消息。
- [x] 3.2 补充或更新 user-services 相关 controller/domain 测试，验证用户 status 错误明细。
- [x] 3.3 对修改的 Go 文件运行 `gofmt`。
- [x] 3.4 在 `common/` 和 `user-services/` 运行 `go test ./...`。
