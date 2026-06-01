## 1. Logger 命名实现

- [x] 1.1 修改 `common/logger` 文件 writer 初始化，将分类日志目标改为 `prefix.yyyy-mm-dd.<level>.log`。
- [x] 1.2 重构 `dailyLumberjackWriteSyncer`，日期变化时关闭旧 logger 并创建指向新日期文件的 `lumberjack.Logger`。
- [x] 1.3 删除不带日期活动文件归档重命名、归档冲突处理和自定义日期归档清理逻辑。
- [x] 1.4 保留 `MaxSizeMB`、`MaxBackups`、`MaxAgeDays` 传递给 `lumberjack`，确保同日大小轮转继续工作。

## 2. 测试更新

- [x] 2.1 更新分类日志测试，验证 Debug/Info/Warn/Error 写入 `prefix.yyyy-mm-dd.<level>.log`。
- [x] 2.2 更新跨天轮转测试，验证新日期第一条日志写入新日期文件，旧日期文件保持原名和内容。
- [x] 2.3 移除或改写归档冲突测试，因为新方案不再重命名不带日期活动文件。
- [x] 2.4 移除或改写自定义日期归档清理测试，改为验证 `lumberjack` 配置仍应用到新日期文件 writer。
- [x] 2.5 保留普通 Error 无 stacktrace 和显式 stacktrace 的测试。

## 3. 规格与文档同步

- [x] 3.1 更新 `openspec/specs/shared-infrastructure/spec.md`，将文件命名长期要求改为每日文件带日期。
- [x] 3.2 更新 `docs/DEVELOPMENT.md`，说明文件日志命名为 `aegiscore-user-services.yyyy-mm-dd.<level>.log`。

## 4. 验证

- [x] 4.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 4.2 在 `common/` 运行 `go test ./...`。
- [x] 4.3 在 `user-services/` 运行 `go test ./...`，确认共享 logger 变更未破坏服务运行时。
