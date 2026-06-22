## Context

`common/runtime/observability/metrics/localcache_test.go` 通过注册 `LocalcacheCollector` 到独立 Prometheus registry，再调用 `Gather` 获取 metric families。测试当前将这些 families 用 `github.com/prometheus/common/expfmt` 转成 Prometheus 文本，再用字符串包含断言指标名称、labels 和数值。

这种写法让测试依赖文本 exposition format，但实际测试目标是 collector 产出的结构化 metric family。`common` 已经直接依赖 `github.com/prometheus/client_golang` 和 `github.com/prometheus/client_model`，因此可以直接对 `dto.MetricFamily`、`dto.Metric` 和 label/value 字段进行断言，无需为了测试文本格式保留 `github.com/prometheus/common` 直接依赖。

本 change 只影响 `common` 模块的 localcache metrics 测试和可能的 module metadata；不影响 `user-service`、`deployments`、HTTP API、数据库 migration、OpenAPI 生成物、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 移除 `localcache_test.go` 对 `github.com/prometheus/common/expfmt` 的直接 import。
- 使用 `prometheus.Gatherer.Gather()` 返回的 `dto.MetricFamily` 结构断言 localcache metrics。
- 保持测试继续覆盖 `cache`、`result`、`event` labels 和对应 value。
- 让 `common/go.mod` 不再因为该测试文件把 `github.com/prometheus/common` 作为直接依赖声明。
- 运行 `common/runtime/observability/metrics` 包测试验证 collector 行为不变。

**Non-Goals:**

- 不修改 `LocalcacheCollector` 生产实现。
- 不改变 Prometheus 指标名称、label key、label value、metric type 或数值。
- 不调整 localcache runtime 统计语义。
- 不移除 `client_golang` 间接依赖带来的 `github.com/prometheus/common` 传递依赖或 `go.sum` 中仍被依赖图需要的校验记录。
- 不修改 user-service 路由、OpenAPI、部署清单、Prometheus scrape、alert 或 Grafana dashboard。

## Decisions

1. 使用 `dto.MetricFamily` 断言，而不是继续经由 expfmt 文本断言。

   - 理由：`Gather` 已经返回 typed metric families，测试可以直接按 name、label pairs 和 counter/gauge value 查询目标 metric，避免引入文本序列化依赖。
   - 备选：保留 `expfmt.MetricFamilyToText` 并只调整断言 helper。该方案无法移除直接依赖，也继续把测试目标绑定到文本输出格式。

2. 在测试文件内新增小型结构化断言 helper。

   - 理由：localcache 测试需要多次按 metric family 名称和 label 组合定位 value，helper 可以减少重复并让失败信息包含可定位的 metric 与 labels。
   - 备选：在测试用例内手写遍历逻辑。该方式会让每个断言样板过多，后续新增 label 组合时更容易遗漏失败上下文。

3. 仅调整 `common` 模块依赖元数据。

   - 理由：`expfmt` 直接 import 删除后，若 `go mod tidy` 判断 `github.com/prometheus/common` 不再是 direct require，应让 `common/go.mod` 反映真实直接依赖关系。
   - 备选：手工保留 direct require。该方式与依赖清理目标冲突，也会让 future tidy 继续产生 drift。

4. 不改动 collector production code。

   - 理由：验收要求明确不改变生产实现或指标契约；测试重构应只验证同一输出结构。
   - 备选：通过生产 helper 暴露内部 desc 或收集逻辑供测试复用。该方式会为了测试便利扩大 production API，不符合 common runtime primitive 边界。

## Risks / Trade-offs

- [Risk] 结构化 helper 若只检查部分 labels，可能丢失原文本断言覆盖面 → Mitigation: helper 必须按完整 label map 匹配，并在 tasks 中要求覆盖 `cache`、`result`、`event` labels 与 value。
- [Risk] Counter 与 Gauge value 字段不同，错误读取会造成误判 → Mitigation: helper 根据 family/metric 的实际字段读取 counter 或 gauge，并在缺少目标 value 时失败。
- [Risk] `go mod tidy` 可能保留 `github.com/prometheus/common` 的间接校验记录 → Mitigation: 验收只要求不再作为测试代码导致的 direct require；不把传递依赖从依赖图中强删。
- [Risk] 修改测试 helper 可能隐藏多个 cache 同时注册的断言 → Mitigation: 保留 `TestLocalcacheCollectorAllowsMultipleCaches`，按不同 `cache` label 分别断言 value。

## Migration Plan

1. 修改 `common/runtime/observability/metrics/localcache_test.go`，删除 `strings` 与 `expfmt` 文本输出 helper。
2. 添加基于 `prometheus.Gatherer` 和 `dto.MetricFamily` 的 metric 查找与断言 helper。
3. 将现有字符串包含断言改为结构化断言，覆盖请求、加载、singleflight、写入、驱逐和容量指标。
4. 在 `common` 模块运行 `go mod tidy`，检查 `common/go.mod` 中 direct require 是否清理。
5. 在 `common` 模块运行 `go test ./runtime/observability/metrics`。

回滚方式：本 change 只涉及测试和 module metadata。如发现测试重构有问题，可回退 `localcache_test.go` 与 `common/go.mod`、`common/go.sum` 的对应 diff，不涉及运行时迁移或数据回滚。

## Open Questions

- 无。
