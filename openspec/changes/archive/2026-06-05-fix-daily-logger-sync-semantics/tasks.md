## 1. 实现

- [x] 1.1 修改 `common/runtime/logger/daily_writer.go`，使 `dailyLumberjackWriteSyncer.Sync()` 不关闭当前 `lumberjack.Logger`，并保持 nil logger 时返回成功。
- [x] 1.2 确认 `rotateLocked()` 在日期变化时仍关闭旧 writer 并创建新日期 writer，不改变文件命名和 lumberjack 配置。

## 2. 测试

- [x] 2.1 在 `common/runtime/logger` 同包测试中增加回归用例：写入日志后调用 `Sync()`，再继续写入同一日期日志应成功且内容落到当前日期文件。
- [x] 2.2 保留或补充按日轮转测试，验证日期变化后旧日期文件保留、新日期文件接收新日志。

## 3. 验证

- [x] 3.1 对修改的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `common/` 模块执行 `go test ./...`。
- [x] 3.3 确认本变更未修改 HTTP API、配置格式、数据库 schema、Ent 生成代码或 Atlas migration。
