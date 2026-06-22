## 1. 测试断言重构

- [x] 1.1 在 `common/runtime/observability/metrics/localcache_test.go` 中删除 `github.com/prometheus/common/expfmt` import 和文本格式转换 helper。
- [x] 1.2 添加基于 `prometheus.Gatherer.Gather()` 与 `dto.MetricFamily` 的结构化 metric 查找 helper。
- [x] 1.3 将 localcache metrics 断言改为按 metric family 名称、完整 label map 和 counter/gauge value 验证。
- [x] 1.4 保留 `TestLocalcacheCollectorExportsStats` 对 requests、loads、singleflight、writes、evictions、capacity 指标的覆盖。
- [x] 1.5 保留 `TestLocalcacheCollectorAllowsMultipleCaches` 对多个 cache label 隔离的覆盖。

## 2. 依赖整理

- [x] 2.1 在 `common` 模块运行 `go mod tidy`，让 module metadata 反映删除 `expfmt` 直接 import 后的依赖关系。
- [x] 2.2 检查 `common/go.mod`，确认 `github.com/prometheus/common` 不再作为测试代码导致的 direct require 存在。
- [x] 2.3 检查 `common/go.sum` diff，确认没有手工删除仍被传递依赖需要的 checksum。

## 3. 验证与收尾

- [x] 3.1 在 `common` 模块运行 `go test ./runtime/observability/metrics`。
- [x] 3.2 运行 `rg -n "expfmt|prometheus/common" common/runtime/observability/metrics/localcache_test.go common/go.mod`，确认测试文件不再 import `expfmt`，且 `common/go.mod` 不再 direct require `github.com/prometheus/common`。
- [x] 3.3 检查 `git diff`，确认没有修改 `LocalcacheCollector` 生产实现、Prometheus 指标名称、label、类型或数值。
- [x] 3.4 实现完成后将对应 tasks checkbox 更新为 `- [x]`，并确认 `openspec status --change remove-localcache-metrics-expfmt-test-dependency` 状态正常。
