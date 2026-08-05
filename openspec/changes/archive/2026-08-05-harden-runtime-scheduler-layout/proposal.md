## Why

`common/runtime/scheduler` 已完成执行管线简化，但 `lifecycle.go` 仍同时承载构造、注册、启停和近两百行使用说明，与 `localcache`、`workerpool` 当前按 package 文档、可执行示例和单一职责文件组织的方式不一致，增加了定位生命周期与并发边界的成本。

`Scheduler.Add` 还只复制 `Job` 最外层结构，默认值归一化会直接修改调用方提供的 `LockPolicy` 与 `RenewPolicy`，注册后调用方继续修改这些指针也会改变已注册任务配置并可能造成 data race。部分 duration 字段使用 `<= 0` 填充默认值，使负数错误配置被静默接受，与公开说明的零值默认语义不一致。

## What Changes

- 将 scheduler 的完整使用契约迁入 `doc.go`，补充可编译、可运行的公开 API examples，并按构造、注册和生命周期职责拆分同 package 文件。
- 在 `Add` 校验和归一化前防御性复制 `LockPolicy` 与 `RenewPolicy`，使 scheduler 保存独立的不可变任务配置快照。
- 只允许零值触发 lock TTL、renew interval、renew timeout 和 Redis retry interval 默认值；负数配置返回 `ErrInvalidLock`。
- 补充配置所有权、严格校验、公开生命周期和 Redis locker 的测试与示例。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 收紧 scheduler 的配置快照所有权和 duration 校验行为，并在不改变公开 API 的前提下整理 package 文档与实现职责。

## Impact

- Go 代码：`common/runtime/scheduler/` 及其测试。
- 共享契约：不新增、删除或重命名导出符号；此前被静默默认化的负数 lock/retry duration 将改为返回 `ErrInvalidLock`。
- 调用方：当前仓库没有 scheduler 生产注册者，无需迁移；合法零值默认配置和正数显式配置保持不变。
- 不影响 HTTP API、数据库 schema/migration、Ent/OpenAPI 生成物、部署清单、观测指标名称、日志字段或安全边界。
