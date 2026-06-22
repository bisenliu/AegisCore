## Why

`common/runtime/localcache/cache.go` 同时承载错误变量、公开类型和核心缓存实现，文件职责偏宽，不利于快速定位稳定 API 与实现细节。
本次变更通过包内拆分提升可读性和维护性，同时保持 `localcache` 的导出 API、错误变量和运行时行为完全不变。

## What Changes

- 将 `common/runtime/localcache/cache.go` 中的错误变量迁移到同包 `errors.go`。
- 将 `Loader`、`CloneFunc`、`Config`、`Stats` 和 `StatsSource` 迁移到同包 `types.go`。
- 保留 `cache.go` 中的 `Cache` 实现、构造函数和方法逻辑。
- 保持 `package localcache`、导出符号、错误值语义、Ristretto 配置、TTL、singleflight、stats 和 `Close` 行为不变。
- 运行 `common/runtime/localcache` 相关测试验证行为未变化。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- 无。本次仅重组 `shared-platform-primitives` 下 `common/runtime/localcache` 的包内文件，不改变主规格需求或稳定行为。

## Impact

- 影响代码：`common/runtime/localcache/cache.go`、新增 `common/runtime/localcache/errors.go`、新增 `common/runtime/localcache/types.go`。
- 不影响外部 API、数据库 schema、OpenAPI、部署资产、观测指标语义或调用方代码。
- 验证范围：优先运行 `go test ./runtime/localcache` 或等价的 `common/runtime/localcache` 包测试；必要时运行 `make common-test`。
