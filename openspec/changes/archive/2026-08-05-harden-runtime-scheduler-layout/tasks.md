## 1. 固化变更契约

- [x] 1.1 完成 proposal、design 和 `shared-platform-primitives` spec delta，明确公开 API 不变、配置快照所有权和负值校验
- [x] 1.2 运行 `openspec validate harden-runtime-scheduler-layout` 并确认 apply artifacts 完整

## 2. 硬化 scheduler 配置

- [x] 2.1 在 `Add` 归一化入口防御性复制 `LockPolicy` 与 `RenewPolicy`，保持调用方对象不变并隔离注册后的修改
- [x] 2.2 仅对零值应用 lock、renew 和 retry duration 默认值，负数统一返回 `ErrInvalidLock`
- [x] 2.3 增加配置快照、默认值与负值拒绝回归测试

## 3. 整理 package 职责与文档

- [x] 3.1 将完整配置、执行、锁、关闭、观测和能力边界迁入 `doc.go`
- [x] 3.2 将构造、注册和生命周期按同 package 文件拆分，保持导出 API 与执行管线不变
- [x] 3.3 增加公开 scheduler 生命周期、Redis locker 与 context 取消 executable examples

## 4. 验证与交付

- [x] 4.1 执行 `gofmt`、`cd common && go vet ./runtime/scheduler`、普通测试和 `go test -race ./runtime/scheduler ./runtime/observability/metrics`
- [x] 4.2 运行 `openspec validate harden-runtime-scheduler-layout`、`openspec list --specs`、`openspec validate --specs`、`make user-service-architecture-lint` 和 `git diff --check`
- [x] 4.3 检查 `git status --short`，只暂存本次预期 change 与 scheduler 文件
- [x] 4.4 在预期变更已暂存后运行 `make lint`，成功后勾选本任务
- [x] 4.5 在预期变更已暂存后运行 `make verify`，成功后勾选本任务并复核 staged diff
