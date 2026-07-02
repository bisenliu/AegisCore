## 1. Scheduler 文件拆分

- [x] 1.1 盘点 `common/runtime/scheduler/scheduler.go` 中的导出类型、生命周期方法、任务执行方法、锁续租方法和校验方法，确认移动边界不改变函数签名。
- [x] 1.2 新增职责明确的源码文件，例如 `types.go`、`lifecycle.go`、`executor.go`、`renew.go`、`validation.go`，避免新增 `utils.go` 或 `helpers.go` 兜底文件。
- [x] 1.3 将公开配置类型、`Scheduler` 类型和常量移动到类型文件，保持 `package scheduler` 和导出符号不变。
- [x] 1.4 将 `New`、`Start`、`AddJob`、`RemoveJob`、`Shutdown` 和停止状态检查移动到生命周期相关文件，保持 cron parser、logger、metrics、root context 和 jobs map 初始化语义不变。
- [x] 1.5 将 `runJob`、global gate、lock acquire、unlock 和 auto renew 相关逻辑移动到执行或续租相关文件，保持 local overlap、global concurrency、lock mode、renew failure、panic recovery 和 metrics 行为不变。
- [x] 1.6 将 `validateJob` 移动到校验相关文件，保持默认值归一化和错误包装语义不变。

## 2. 测试与规格同步

- [x] 2.1 运行 `gofmt` 覆盖 `common/runtime/scheduler/*.go`，确认拆分后的 import 无漂移。
- [x] 2.2 运行 `go test ./runtime/scheduler`（在 `common/` 目录）或等价 scheduler 包测试，确认 scheduler 与 Redis locker 现有行为全部通过。
- [x] 2.3 如拆分导致测试可读性明显下降，按职责小幅调整 `scheduler_test.go` 或 `lock_test.go`，不得新增仅服务测试的生产接口、分支或适配层。
- [x] 2.4 运行 `make user-service-architecture-lint`，确认 OpenSpec delta 和架构边界文档约束仍可通过。

## 3. 最终验证

- [x] 3.1 检查 `git diff`，确认没有导出 API、错误变量、日志语义、OpenAPI、数据库 migration、部署资产或 user-service feature 的非预期变更。
- [x] 3.2 将本次预期代码、OpenSpec artifacts 和相关文档变更加到暂存区。
- [x] 3.3 运行 `make lint`，未通过时修复后重新运行。
- [x] 3.4 运行 `make verify`，未通过时修复后重新运行，并确认不存在未暂存的预期 diff 阻塞最终检查。
- [x] 3.5 全部验证通过后，将已完成任务 checkbox 更新为 `- [x]`，并准备执行 `/opsx:archive split-scheduler-files`。
