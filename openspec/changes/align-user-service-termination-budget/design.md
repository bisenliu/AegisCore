## Context

user-service 通过 `user-service/cmd/serve.go` 为 `app.Stop()` 创建 `runtime.lifecycle.stop_timeout` 控制的 context，默认值由 `common/runtime/config/defaults.go` 提供且在 `user-service/configs/config.yaml` 中显式声明为 120 秒。Fx 按注册逆序串行执行 `OnStop` hook；所有 hook 共享这一绝对 deadline，前序 hook 消耗的时间会减少后序 hook 的可用预算。

当前默认关闭组件和预算边界如下：

| 组件 | 当前关闭边界 | 预算含义 |
|---|---:|---|
| HTTP server 与 active handler drain | 25 秒 | 在 Fx 剩余 context 内再建立 25 秒子 deadline；失败后强制关闭连接并等待 handler |
| auth session purge workerpool | 30 秒 | 在 Fx 剩余 context 内再建立 30 秒子 deadline，超时后取消 pool task |
| RBAC policy watcher | Fx 剩余预算 | 取消 Pub/Sub/补偿循环并等待 goroutine 退出 |
| pprof server | Fx 剩余预算 | 调用 `http.Server.Shutdown(ctx)` |
| tracing provider | Fx 剩余预算 | flush 并关闭 OpenTelemetry provider；OTLP exporter 的 5 秒 timeout 是单次 exporter I/O 边界，不是独立进程停止预算 |
| Ent client、PostgreSQL pool、Redis client | Fx 剩余预算 | 依 Fx 逆序关闭 service adapter 与底层连接资源；关闭调用没有额外可累加的配置化 timeout |
| logger | Fx 剩余预算 | 最后同步 zap 输出；stdout/stderr 不支持 `fsync` 的预期错误继续忽略 |

组件子预算不能相加后替代 120 秒 Fx 总预算：子 context 均受父 Stop context 限制，而没有独立 timeout 的 hook 也会消耗同一剩余时间。当前 Kubernetes manifest 和 Helm values 都设置 35 秒 grace，只覆盖 HTTP 的 25 秒局部预算，无法覆盖 Fx 全链路。Kubernetes 的 grace 还包含 kubelet 执行 `preStop`、向进程发出 `SIGTERM` 以及最终 `SIGKILL` 前的调度时间；当前部署未配置 `preStop`，但仍需为未来 hook 与平台抖动保留显式余量。

本 change 跨越 `user-service` 默认配置、`deployments` 清单/Chart、CI 可达测试和 `docs/openspec` 契约。`common` 仅作为现有 120 秒默认值和 Fx lifecycle primitive 的来源，不新增业务或部署策略 helper；`internal/shared`、`internal/integration` 和各 feature 业务语义不承载部署预算规则。前置 `fix-user-service-internal-shutdown` 已完成实现与校验，但当前仍在活动 change 目录，实施本 change 前必须先按 OPSX 流程归档它。

## Goals / Non-Goals

**Goals:**

- 让原生 Kubernetes 与 Helm 默认 grace 严格大于 120 秒 Fx Stop 总预算，并保留可审查、可自动验证的 30 秒平台余量。
- 固定 `deployment grace >= runtime.lifecycle.stop_timeout + 30s` 的默认配置契约，并验证原生 Kubernetes 与 Helm 默认值一致。
- 用结构化 YAML 解析的自动测试覆盖仓库真实默认文件、低于约束值和两个部署入口漂移等情况，使 CI 能稳定阻止回归。
- 在部署文档与 delta specs 中区分 Fx 总预算、局部组件预算和平台余量，保留 Fx 逆序串行停止语义。

**Non-Goals:**

- 不压缩 120 秒 Fx Stop 默认预算，不修改任何组件关闭 timeout，也不并行执行 Fx `OnStop` hook。
- 不承诺每次关闭耗时 120 秒；无待处理工作时各 hook 仍应立即返回。
- 不改变业务 API、OpenAPI、认证/RBAC 语义、数据库 schema、Atlas migration、依赖版本、metrics 指标或日志字段契约。
- 不新增 `preStop` hook；未来若新增，必须在 30 秒平台余量内证明上界，或同步提高 grace 和自动校验基线。

## Decisions

### Decision: 保持应用 Stop 预算 120 秒，将部署默认 grace 调整为 150 秒

原生 `deployments/k8s/user-services/deployment.yaml` 和 Helm `deployments/helm/aegiscore-user-services/values.yaml` 的默认 `terminationGracePeriodSeconds` 统一设为 150。约束表达为：

```text
terminationGracePeriodSeconds >= stop_timeout_seconds + platform_margin_seconds
150 >= 120 + 30
```

因此 grace 严格大于应用 Stop 总预算，并为 kubelet 调度、信号传递、未来受控 `preStop` 和网络抖动保留 30 秒。选择 150 秒而不是压缩应用预算，是因为当前 120 秒是所有逆序串行 hook 的全局上界；在没有生产关闭耗时数据和全部 hook 累计证明前缩短它会增加后序 datastore/logger hook 被取消的风险。也不选择只提高到 121 秒，因为 1 秒不能构成可运营的平台余量。

### Decision: 自动校验仓库真实默认文件，而不是复制常量快照

在 user-service 的 Go 测试边界新增配置一致性测试，使用 YAML parser 读取：

- `user-service/configs/config.yaml` 的 `runtime.lifecycle.stop_timeout`；
- `deployments/k8s/user-services/deployment.yaml` 的 `spec.template.spec.terminationGracePeriodSeconds`；
- `deployments/helm/aegiscore-user-services/values.yaml` 的 `deployment.terminationGracePeriodSeconds`。

测试 helper 接受解析后的输入并固定最小平台余量 30 秒，断言两种部署默认值相等且均不小于应用 Stop 预算加余量。表驱动负例覆盖 grace 小于最低值、仅一个入口变更和无效/缺失字段，使失败原因可定位。该测试随 `make user-service-test`、`make test` 和 CI/`make verify` 执行；`make user-service-architecture-lint` 仍独立验证目录边界，避免让架构脚本依赖 `yq`、Ruby 或脆弱文本匹配。

备选方案是在 shell architecture lint 中用正则提取三个值，但它无法可靠处理 YAML 结构、单位与字段移动；另一个方案是新增生产期配置校验，让应用读取 Kubernetes grace，但 Pod spec 不属于进程配置且运行时通常不可见，因此不采用。

### Decision: 以 Fx 全局 deadline 作为部署约束源

关闭预算文档以 `runtime.lifecycle.stop_timeout` 为应用总预算源。HTTP 25 秒、workerpool 30 秒、OTLP 5 秒等仅用于说明 hook 内部边界，不能用于计算比 Fx 总预算更小的部署 grace。Fx 继续逆序串行调用 hook，所有 hook 继续接收同一个 Stop context 或其派生子 context。

备选方案是把所有局部 timeout 求和作为 grace 基线，但没有额外 timeout 的 watcher/tracing/datastore hook 无法形成有限加和，而且局部 context 受 120 秒父 deadline 限制；该算法既高估部分路径又可能漏算无局部 timeout 的路径。并行执行 hook 虽可能缩短墙钟时间，却会破坏 HTTP、worker、watcher、Ent/datastore、tracing 和 logger 的资源依赖顺序，因此明确排除。

### Decision: 同步部署说明与 OpenSpec，不扩散策略到 common

更新 Kubernetes/Helm 值旁注释及相关部署 README，记录 120 + 30 = 150 的默认预算、正常关闭可提前完成、`preStop` 必须计入余量的要求。`delivery-operations` delta 负责默认值、配置一致性门禁和发布行为；`runtime-observability` delta 负责 Fx 总预算与组件关闭/flush 的可观测边界。

不在 `common/runtime/config` 中引入 Kubernetes/Helm 字段或平台余量常量，因为该策略属于 user-service 交付资产。Go 代码只新增测试，不改变正式构建；`internal/shared` 和 `internal/integration` 均不受影响。无需更新 OpenAPI 生成物、Ent 代码、SQL migration、Prometheus alerts 或 Grafana dashboards。

## Risks / Trade-offs

- [Risk] 卡死 Pod 的强制回收时间从 35 秒增长到 150 秒，可能降低故障替换速度 → Mitigation：120 秒是显式应用停止上界，正常关闭会立即退出；故障恢复策略继续依赖多副本、readiness、PDB 和 rollout 配置，后续只能基于观测数据和完整预算证明调整。
- [Risk] 测试固定 30 秒余量后，修改应用 Stop 预算或新增 `preStop` 会触发交付失败 → Mitigation：这是预期门禁；同一 change 必须更新两个部署默认值、预算说明和测试基线并给出余量依据。
- [Risk] Helm 环境覆盖值仍可能小于默认契约 → Mitigation：本 change 保证仓库默认值和原生清单一致，并在 README 明确环境覆盖责任；集群 admission policy 或部署平台校验属于后续可选强化，不在本次范围。
- [Risk] Fx hook 实际注册顺序随依赖图变化 → Mitigation：约束使用覆盖整个 `app.Stop` 的总 deadline，不依赖固定逐项顺序；盘点只记录资源关系和预算来源，不把当前构造顺序固化为外部契约。

## Migration Plan

1. 先确认 `fix-user-service-internal-shutdown` 校验通过并归档，使内部故障和外部信号都进入统一的 `app.Stop` 路径。
2. 新增结构化配置一致性测试及负例，先证明当前 35 秒配置会失败。
3. 将原生 Kubernetes 与 Helm 默认 grace 同步改为 150 秒，更新预算注释和部署 README，使测试转为通过。
4. 运行相关 Go 测试、Helm/Kubernetes 渲染检查、`make user-service-architecture-lint` 和 `openspec validate align-user-service-termination-budget`。
5. 暂存仅属于本 change 的预期文件，检查 staged diff 后运行 `make lint` 与 `make verify`。

发布时沿用现有 migration、RBAC seed、HTTP rollout 顺序。该变化不需要数据或配置迁移；部署新清单后，Pod 终止宽限期随 Deployment rollout 生效。回滚可将原生 Kubernetes 与 Helm grace 一并恢复，但会重新暴露 35 秒提前 `SIGKILL` 风险，因此只应在确认更长 terminating Pod 阻断发布且具备替代保护时执行。

## Open Questions

无。平台余量固定为 30 秒，应用 Stop 默认预算保持 120 秒，两个默认部署入口统一使用 150 秒。
