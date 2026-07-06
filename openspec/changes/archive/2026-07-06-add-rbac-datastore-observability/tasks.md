## 1. RBAC Enforce 指标

- [x] 1.1 扩展 `user-service/internal/features/permission/application` 的 feature-local metrics interface，新增 Enforce 结果与耗时记录方法，并重新生成 no-op metrics 文件。
- [x] 1.2 在 `user-service/internal/features/permission/metrics.go` 注册 `aegiscore_user_service_rbac_enforce_total` counter 和 `aegiscore_user_service_rbac_enforce_duration_seconds` histogram，标签仅使用 `result`、`method`、`route_template`。
- [x] 1.3 在 `user-service/internal/features/permission/application/authorization` 的 Enforce 入口记录 `allow`、`deny`、`error` 和 latency，保持非法 subject、engine error、deny、allow 的现有返回语义不变。
- [x] 1.4 增加 RBAC Enforce metrics 单元测试，覆盖 allow、deny、invalid subject、engine error 和 label 禁止高基数字段。

## 2. Redis OpenTelemetry tracing

- [x] 2.1 为 `common` 或 workspace 添加 go-redis v9 OpenTelemetry instrumentation 依赖，确保版本与当前 `github.com/redis/go-redis/v9` 和 OpenTelemetry 版本兼容。
- [x] 2.2 在 `common/runtime/datastore` Redis client 创建点统一安装 OpenTelemetry hook，不在 user-service provider 重复安装。
- [x] 2.3 增加 Redis tracing 测试，验证带有效 trace context 的 Redis 命令产生 client span，tracing no-op 时命令结果不变。
- [x] 2.4 运行 `go test ./runtime/datastore/...` 或相关 common 包测试，确认 Redis 生命周期、ping 和 datastore provider 行为不变。

## 3. Ent 查询 tracing 与 metrics

- [x] 3.1 在 `user-service/internal/providers/ent.go` 或相邻服务级观测文件中实现 Ent query interceptor，产生低基数 query span，不修改 `user-service/ent/` 生成代码。
- [x] 3.2 在服务级 metrics 注册 Ent query latency histogram 和 query error counter，标签使用稳定低基数 entity/query/result 枚举，不包含 raw SQL、参数、用户标识或原始错误。
- [x] 3.3 在 Ent client 构造路径安装 query tracing 与 metrics，并在 metrics 或 tracing provider 禁用时使用 no-op 行为保持业务结果不变。
- [x] 3.4 增加 Ent provider 测试，覆盖成功 query span、query latency histogram、失败 query error counter 和禁用观测行为。

## 4. 部署观测资产

- [x] 4.1 更新 Prometheus alert、Grafana dashboard 或 metrics load 验证资产，直接消费新增 RBAC Enforce 和 Ent query 指标，不保留旧指标名、旧 label 或兼容 PromQL。
- [x] 4.2 运行 `make compose-dashboard-check` 或对应观测资产检查命令，确认生成物无 drift。

## 5. 规格与验证

- [x] 5.1 运行 `make user-service-architecture-lint`，确认 common、permission feature、provider 和 observability 边界符合架构约束。
- [x] 5.2 运行相关包测试：permission authorization/metrics、common datastore Redis、user-service providers Ent tracing/metrics。
- [x] 5.3 将本次预期代码、依赖、部署观测资产和 OpenSpec artifacts 加入暂存区。
- [x] 5.4 运行 `make lint`，失败时修复后重新运行。
- [x] 5.5 运行 `make verify`，失败时修复 drift 或测试问题后重新运行。
- [x] 5.6 确认 `git diff --cached` 仅包含本 change 预期内容，未暂存 diff 不包含本 change 遗漏文件。
