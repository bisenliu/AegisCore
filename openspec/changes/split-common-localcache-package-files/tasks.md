## 1. 结构拆分

- [x] 1.1 检查 `common/runtime/localcache/cache.go` 中现有错误变量、公开类型和 `Cache` 实现声明，确认仅做声明移动。
- [x] 1.2 新增 `common/runtime/localcache/errors.go`，迁移 `ErrNameRequired`、`ErrCapacityRequired`、`ErrTTLRequired`、`ErrKeyStringRequired`、`ErrLoaderRequired` 和 `ErrClosed`，保持错误字符串不变。
- [x] 1.3 新增 `common/runtime/localcache/types.go`，迁移 `Loader`、`CloneFunc`、`Config`、`Stats` 和 `StatsSource`，保持类型签名、字段和注释不变。
- [x] 1.4 收敛 `common/runtime/localcache/cache.go`，仅保留 `defaultBufferItems`、`Cache` 结构、构造函数和方法实现，并清理 import。

## 2. 验证

- [x] 2.1 对 `common/runtime/localcache` 相关 Go 文件运行 `gofmt`。
- [x] 2.2 在 `common/` 模块内运行 `go test ./runtime/localcache`，确认 localcache 测试通过。
- [x] 2.3 检查 diff，确认未修改调用方代码、API、Ristretto 配置、TTL、singleflight、stats 或 `Close` 行为。
