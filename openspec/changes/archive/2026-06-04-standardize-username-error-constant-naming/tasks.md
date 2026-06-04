## 1. 命名审查

- [x] 1.1 在仓库内搜索 `MsgInvalidUserName`、`UserName` 和 `userName`，记录需要修改的内部 Go 符号候选。
- [x] 1.2 排除外部契约字符串、JSON/query 字段、数据库字段、配置 key、migration 历史和 Ent 生成代码中的无需修改项。

## 2. 实现重命名

- [x] 2.1 将 `user-services/internal/errmsg/messages.go` 中的 `MsgInvalidUserName` 重命名为 `MsgInvalidUsername`，保持中文错误消息文本不变。
- [x] 2.2 同步更新所有引用 `MsgInvalidUserName` 的 Go 文件和测试文件。
- [x] 2.3 如审查发现其他低风险 `UserName` 或 `userName` 内部 Go 符号，同步改为 `Username` 风格并更新引用。

## 3. 验证

- [x] 3.1 对修改过的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `user-services/` 运行 `go test ./...`，确认用户服务编译和测试通过。
- [x] 3.3 如审查范围涉及 `common/`，在 `common/` 运行 `go test ./...`。
- [x] 3.4 汇总修改清单，说明每处命名修改原因、影响范围和外部行为未改变。
