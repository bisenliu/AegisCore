## Why

`common/runtime/observability/metrics/localcache_test.go` 当前直接 import `github.com/prometheus/common/expfmt`，只是为了把 Prometheus 文本输出解析回指标断言。这让 `common/go.mod` 因测试文本格式解析而声明额外直接依赖，增加了 common 模块的依赖表面积。

本 change 将 localcache metrics 测试改为直接使用 Prometheus `Gather` 返回的 `dto.MetricFamily` 结构断言指标名称、labels 和 value，在不改变生产指标契约的前提下移除 `expfmt` 测试依赖。

## What Changes

- 修改 `common/runtime/observability/metrics/localcache_test.go`，删除对 `github.com/prometheus/common/expfmt` 的直接 import。
- 使用 Prometheus registry `Gather` 返回的 `dto.MetricFamily`、`Metric`、`LabelPair` 和 metric value 字段完成断言。
- 保持 localcache collector 现有指标名称、label key、类型和数值语义不变。
- 必要时运行 `go mod tidy`，让 `common/go.mod` 不再因为该测试文件直接声明 `github.com/prometheus/common`。
- 运行 `common/runtime/observability/metrics` 相关 Go 测试验证覆盖面不丢失。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 保持 localcache metrics 指标契约不变，并将测试验证方式从 expfmt 文本解析收敛到 Prometheus `dto.MetricFamily` 结构断言。

## Impact

- 影响代码范围：`common/runtime/observability/metrics/localcache_test.go`。
- 可能影响依赖文件：`common/go.mod`、`common/go.sum`。
- 不影响 `LocalcacheCollector` 生产实现、HTTP API、数据库 schema、OpenAPI、部署清单、Prometheus 指标名称、label、类型或数值。
- 不移除 `client_golang` 间接依赖带来的 `github.com/prometheus/common` 传递依赖；只清理 common 测试代码导致的直接声明。
- 验证范围：运行 `go test ./runtime/observability/metrics` 于 `common` 模块，并检查 `localcache_test.go` 不再 import `expfmt`。
