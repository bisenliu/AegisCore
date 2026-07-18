## Context

`common/runtime/fxgraph.Generate` 当前接收 Fx options 后通过 `fx.New` 与 `fx.Populate(&graph)` 获取 `fx.DotGraph`。在 user-service 中，`fxgraph` 命令复用完整 `bootstrap.AppModule`，该 module 包含 timezone、runtime dependency metrics、route 注册、RBAC lifecycle、HTTP server 和 pprof server 等 `fx.Invoke`。由于 `fx.New` 会执行全部 Invoke，graph 生成会迫使 Fx 构造传递依赖，可能触发 logger、tracing、SQL、Redis、Ent、本地缓存、workerpool、Gin engine 以及进程级 timezone/Gin mode 等运行时行为。

该 change 影响 `common/runtime/fxgraph` 的 helper 职责边界，以及 user-service composition root 的 Fx module 组织方式。正式 `serve` 的启动、停止、HTTP API、数据库 schema、OpenAPI 生成物、部署清单和 RBAC 行为不应改变。

## Goals / Non-Goals

**Goals:**

- 将 user-service Fx module 拆分为 `WiringModule` 与 `RuntimeModule`，使正式 `AppModule` 仍保持完整运行时行为。
- 让 `fxgraph` 命令只使用无副作用 wiring graph 或专用 graph root 生成 DOT。
- 保留 graph 中对 auth、user、role、permission、providers、router 和关键 metrics 依赖边的诊断价值。
- 增加测试验证 graph 生成不会连接 PostgreSQL、Redis、OTLP，不创建 workerpool、本地缓存或 tracing exporter 后台资源，不注册真实 route/runtime metrics，不修改 timezone 或 Gin mode。
- 保持 `common/runtime/fxgraph` 业务中立，不导入 user-service 私有配置或 feature 包。

**Non-Goals:**

- 不重构 feature 内业务分层、RBAC policy sync、auth session、OpenAPI 生成或数据库 migration。
- 不改变 `serve`、`rbac` 或 `fxgraph` 的公开 CLI 名称、flag、默认配置路径、退出码和输出语义。
- 不新增测试专用生产 API、全局可变 hook 或 `ForTest` 构造器。
- 不要求 graph 命令展示 runtime `fx.Invoke` 的真实执行效果；需要展示时只能使用无副作用 graph root。

## Decisions

### Decision: 拆分 wiring 与 runtime module

将当前生产 `AppModule` 拆分为 `WiringModule`、`RuntimeModule` 和组合后的 `AppModule`。`WiringModule` 只包含 provider 注册、feature module、providers wiring、HTTP/pprof server constructor 等不应主动启动运行时行为的装配；`RuntimeModule` 包含 timezone 初始化、runtime metrics 注册、route 注册、RBAC lifecycle、server activation 和其他必须由 Invoke 驱动的运行时激活逻辑。

选择该方案的原因是正式 App 仍通过 `AppModule = WiringModule + RuntimeModule` 保持原有语义，而 graph 命令可直接选择 wiring graph。备选方案是继续复用完整 App 并在 constructors 中添加 graph mode 分支，但这会把开发工具语义扩散到生产 constructor，增加测试专用分支和长期维护成本。

### Decision: graph 命令使用无副作用 root

`user-service/cmd/fxgraph.go` 应使用 `bootstrap.WiringModule` 或 `bootstrap.GraphModule` 生成 DOT，而不是完整 `bootstrap.AppModule`。如果仅使用 wiring module 会丢失关键诊断边，可提供显式 `GraphModule`，但其中的 graph root 必须是无副作用 Invoke 或 Populate 输入，不能注册真实 route、lifecycle hook、外部连接或后台资源。

选择该方案的原因是 graph 命令的安全边界清晰，且不需要通过 stop-only hook 回滚 constructor 副作用。备选方案是在 common helper 中过滤 Invoke，但 Fx option 无法可靠区分服务私有运行时 Invoke 与安全 Invoke，且会让 common 承担服务语义。

### Decision: common helper 保持薄封装

`common/runtime/fxgraph` 继续只负责对传入 Fx option 生成稳定 DOT 文本、错误传播和输出规范化，不负责加载服务配置、替换 provider、注入 fake 资源或理解 user-service module。common helper 的测试应使用业务中立 fixture 验证不会自行要求服务私有输入。

选择该方案的原因是 `common` 必须保持跨服务共享 primitive，不承载 user-service 私有配置、feature provider 或业务 DTO。备选方案是在 common 中提供 graph-specific resource stubs，但这会违反 common 与服务边界。

### Decision: 用可观察测试保护副作用边界

新增测试优先验证行为，而不是依赖实现细节。测试可通过可失败的 provider、局部 fake constructor、进程状态快照和 graph command execution 来证明 graph path 不触发 runtime module。涉及 `TZ`、`time.Local`、Gin mode 或默认 logger 的测试必须保存并恢复状态，且不得并行执行。

选择该方案的原因是 graph 的风险来自隐式副作用，单纯检查 module 名称不足以防回归。备选方案是只做静态 import 或文本检查，但无法发现新增 Invoke 间接触发资源构造。

## Risks / Trade-offs

- [Risk] 拆分 module 时遗漏正式 runtime Invoke，导致 `serve` 不再注册路由、metrics 或 lifecycle。→ Mitigation: 增加正式 `AppModule` 构图测试和现有 HTTP/route/RBAC 测试回归，确认 `serve` 继续使用组合后的 `AppModule`。
- [Risk] wiring graph 去掉 runtime Invoke 后 DOT 诊断信息减少。→ Mitigation: 如需关键边，提供专用无副作用 graph root，明确只用于构图，不执行真实运行时注册。
- [Risk] 为了测试副作用边界引入生产 API 污染。→ Mitigation: 只使用现有依赖注入、package-local `_test.go` fixture、局部 fake 和可观察状态，不新增 `ForTest`、test hook 或全局可变函数。
- [Risk] 进程级状态测试互相污染。→ Mitigation: 测试保存并恢复 `TZ`、`time.Local`、Gin mode 等状态，相关测试不并行运行。
- [Risk] graph 命令仍通过 constructor 间接创建真实资源。→ Mitigation: graph path 不应包含需要真实资源输入的 runtime Invoke，测试中使用会失败的资源 constructor 证明不会被调用。

## Migration Plan

1. 在 user-service bootstrap 中引入 `WiringModule` 和 `RuntimeModule`，将现有 `AppModule` 改为组合两者。
2. 调整 `fxgraph` 命令使用无副作用 graph root。
3. 更新 common/user-service 相关测试，覆盖 graph 无副作用和正式 App 构图不回退。
4. 运行相关 Go 测试、`make user-service-architecture-lint`，最终暂存预期变更后运行 `make lint` 和 `make verify`。

回滚方式：恢复 `fxgraph` 命令使用 `AppModule` 和原有 bootstrap module 组织即可回到旧行为；由于不涉及数据、API、部署或 migration，无需运行时数据迁移。

## Open Questions

- 无。当前需求已明确 graph 命令应优先安全无副作用，若未来必须展示 runtime Invoke，应单独设计无副作用 graph root。
