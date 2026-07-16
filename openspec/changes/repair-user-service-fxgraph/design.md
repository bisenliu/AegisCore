## Context

`user-service` 已有 `fxgraph` 命令和 `common/runtime/fxgraph` 渲染 primitive，但当前诊断命令没有获得正式 App 装配所需的完整基础输入，尤其是 `*serviceconfig.Config`，因此 `go run ./cmd fxgraph` 会在 Fx 图构建阶段确定性失败。与此同时，正式 App 配置装配正在通过 `unify-user-service-app-configuration` 收敛到基础 options builder，如果 `fxgraph` 继续维护独立的 mock option 列表，诊断图会持续偏离运行图。

本 change 涉及 `user-service/cmd`、`user-service` 交付命令或脚本、版本控制的 Fx dependency graph 资产，以及 `common/runtime/fxgraph` 的边界约束。它不改变 HTTP API、数据库 schema、OpenAPI、RBAC、安全策略或生产 server lifecycle。

## Goals / Non-Goals

**Goals:**

- `fxgraph` 命令基于正式 App 的基础 input/options builder 组装依赖图，使缺少 `*serviceconfig.Config` 这类关键输入的问题能被测试和命令稳定暴露。
- 命令层为诊断图提供无外部副作用的替身依赖，包括 service config、派生 runtime config、logger、PostgreSQL/Redis/OTLP 等资源替身或空实现。
- `common/runtime/fxgraph` 保持业务中立，仅负责从 Fx option 或 module 渲染稳定 DOT 文本。
- 测试实际调用 `RenderDOT`，覆盖成功图输出和关键依赖缺失失败路径。
- 更新受版本控制的 Fx dependency graph 资产，并保证生成或 check 目标能发现 drift。

**Non-Goals:**

- 不让 `fxgraph` 连接真实 PostgreSQL、Redis、OTLP 或启动 HTTP server。
- 不在 `common/runtime/fxgraph` 中引入 user-service 配置、feature provider、Ent 或服务私有资源。
- 不改变正式 `serve` 运行图中的业务 provider 行为、生命周期 hook 或外部契约。
- 不新增数据库迁移、OpenAPI 注解或部署发布流程。

## Decisions

1. `fxgraph` 复用正式 App 基础 builder，而不是维护独立 provider 清单。

   这样可以让诊断图与运行图共享同一组基础 option 入口，减少遗漏配置、logger、resource provider 或 feature module 的风险。备选方案是继续手写 `fx.Option` 列表并补上缺失 config，但这只能修复当前失败，无法防止后续 builder 漂移。

2. 服务私有输入留在 `user-service/cmd` 或 user-service 内部装配边界。

   `fxgraph` 需要的 service config、runtime config 派生和资源替身都有 user-service 语义，必须由命令层或服务私有 provider 负责。备选方案是扩展 `common/runtime/fxgraph` 接收或构造这些输入，但这会破坏 common 的业务中立边界。

3. 外部资源使用无副作用替身，而不是真实连接。

   依赖图生成只需要 Fx 类型图和 provider 关系，不需要真实数据库、Redis、OTLP exporter 或 HTTP listener。无副作用替身能在本地、CI 和离线环境稳定运行。备选方案是依赖配置连接真实资源，但会让诊断命令变成环境敏感的集成启动，违背图生成目标。

4. 测试直接调用 DOT renderer 并断言图内容。

   仅断言 option 数量无法证明 Fx 图可构建，也无法发现缺失 `*serviceconfig.Config` 这类关键输入。smoke test 应断言输出非空、包含 AppModule 或顶层 module、包含 auth/user/role/permission 等关键 feature 节点或依赖边，并覆盖删除关键输入时失败。备选方案是只测试命令参数解析，但无法覆盖 Fx 依赖解析。

5. 复用已有生成或 drift 检查目标，缺失时补充服务前缀目标。

   若仓库已有 `user-service-` 前缀的 fxgraph 生成/check 目标，应优先复用并修复。若缺失，应新增带服务名前缀的目标，避免根 Makefile 出现无服务上下文目标。

## Risks / Trade-offs

- [Risk] 无副作用替身与真实 provider 类型不完全一致，导致图和正式 App 仍有局部差异。Mitigation: 替身只替换外部副作用边界，尽量保留正式 App 的基础 builder 和 feature module，测试断言关键节点与依赖边。
- [Risk] Fx DOT 输出格式随 Fx 或渲染实现变化而产生非语义 diff。Mitigation: 保持 `common/runtime/fxgraph` 稳定排序，并通过受控生成/check 目标暴露 drift。
- [Risk] 诊断命令复用正式 builder 后可能引入启动 HTTP server 或连接外部资源的 invoke。Mitigation: `fxgraph` 组装只执行图渲染需要的 dry-run/validation 路径，替换外部资源 provider，不调用生产 server start 行为。
- [Risk] 为测试方便新增生产 test-only hook。Mitigation: 测试基于公开命令构造、基础 builder 和 renderer 行为，不引入 `ForTest`、`testHook` 等正式构建 API。

## Migration Plan

1. 调整 `user-service/cmd` 的 `fxgraph` 装配逻辑，使其复用正式 App 基础 input/options builder 并注入无副作用替身。
2. 更新或补充 `user-service/cmd` 测试，实际调用 `RenderDOT` 并覆盖成功与缺失关键依赖失败路径。
3. 重新生成版本控制中的 Fx dependency graph 资产，并确保服务前缀生成/check 目标通过。
4. 执行 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot`、`cd user-service && go test ./cmd -count=1`、`openspec validate repair-user-service-fxgraph`、`make user-service-architecture-lint`。
5. 完成实现和文档后暂存预期变更，再执行 `make lint` 与 `make verify`。

回滚方式是撤销 `fxgraph` 命令、测试、图资产和交付目标的本次变更。由于不涉及数据库、外部 API 或部署状态，回滚不需要数据迁移。

## Open Questions

- 若仓库已有多个 Fx graph 资产或目标，实施时需要确认哪一个是 authoritative 生成物，并只更新对应受版本控制资产。
