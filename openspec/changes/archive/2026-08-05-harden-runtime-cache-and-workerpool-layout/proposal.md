## Why

`common/runtime/workerpool` 与 `common/runtime/localcache` 已具有明确稳定契约，但核心实现文件同时承载长篇使用说明、公开类型、生命周期、执行或加载状态机，增加了定位并发边界和审查局部修改的成本。

`localcache` 还存在两个可收敛的实现边界：`singleflight` 结果直接通过 `any` 往返会在泛型 value 为接口类型且 loader 返回 nil 时触发类型断言 panic；容量驱逐通过 `ttlcache.OnEviction` 异步回调计数，会为显式失效和 TTL 清理等无关删除启动瞬时 goroutine，并使容量统计延迟可见。

## What Changes

- 在原 package 内按文档、公开类型、执行、生命周期、加载、失效和统计职责拆分 `workerpool` 与 `localcache` 文件，保持导入路径和导出 API 不变。
- 使用包内泛型结果容器承载 `singleflight` 返回值，使接口类型的 nil value 可以安全返回和缓存。
- 在 localcache 发布锁内同步判定并累计容量驱逐，移除 `ttlcache.OnEviction` 异步回调。
- 将长篇使用契约迁入 package 文档，并修正 ants `WithPreAlloc` 只预分配内部队列内存的描述。
- 补充单元测试，覆盖 nil interface 和同步容量统计，并继续覆盖并发加载、强失效和 workerpool 生命周期。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 收紧 localcache 泛型 nil value 与容量驱逐统计的稳定行为，并在不改变 workerpool/localcache 公开 API 的前提下整理包内职责。

## Impact

- Go 代码：`common/runtime/workerpool/`、`common/runtime/localcache/` 及其测试。
- 共享契约：不新增、删除或重命名导出符号；localcache 修复此前会 panic 的泛型边界，并让既有容量驱逐统计同步可见。
- 调用方：`user-service` 无需迁移，现有 auth session purge、token version cache 和 RBAC user role cache 调用保持不变。
- 不影响 HTTP API、数据库 schema/migration、Ent/OpenAPI 生成物、部署清单、观测指标名称、日志字段或安全边界。
