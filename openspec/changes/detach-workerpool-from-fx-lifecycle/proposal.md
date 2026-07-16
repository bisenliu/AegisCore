## Why

`common/runtime/workerpool` 当前把任务池生命周期绑定到 Fx，导致这个跨服务 runtime primitive 无法作为普通 Go 构造器被资源所有者显式创建和关闭。后续 auth 显式生命周期与顶层 Runtime 解耦需要先移除该隐式 Fx 托管边界，避免 `common` primitive 继续携带服务装配框架依赖。

## What Changes

- **BREAKING**: 移除 `workerpool.New` 对 `go.uber.org/fx`、`fx.Lifecycle` 和 `fx.Hook` 的生产依赖，构造器不再接收 lifecycle 参数。
- **BREAKING**: 调用方必须持有 `*workerpool.Pool` 并在资源所有者边界通过公开 `Stop(ctx)` 显式关闭任务池；不保留旧签名、deprecated wrapper 或可选 lifecycle 参数。
- 保持 workerpool 现有并发上限、阻塞提交、任务 context 联动、panic recovery、统计、幂等 drain、StopTimeout 和错误语义不变。
- 移除 `common/runtime/workerpool` 测试中的 `fxtest` fixture 与 Fx lifecycle 断言，改为直接构造、提交、拒绝、drain 和 Stop 行为测试。
- 同步迁移 auth refresh session purge pool 当前调用点；在 user-service 仍使用 Fx 的阶段，仅允许服务私有 composition 直接登记 `pool.Stop`。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 修改 workerpool lifecycle 归属要求，删除由 Fx hook 托管的约束，明确调用方所有权和显式关闭责任。

## Impact

- 影响 `common/runtime/workerpool` 的公开构造器签名、包注释、生命周期实现和单元测试。
- 影响当前直接创建 auth session purge worker pool 的 user-service 调用点，需要由服务私有装配边界在 Fx 生命周期中显式调用 `pool.Stop(ctx)`。
- 影响 Go 依赖：`common/runtime/workerpool` 不再导入 `go.uber.org/fx` 或 `fxtest`。
- 不影响 HTTP API、OpenAPI、数据库 schema、Redis key、session purge 业务语义、worker 数、StopTimeout 配置或部署资产。
