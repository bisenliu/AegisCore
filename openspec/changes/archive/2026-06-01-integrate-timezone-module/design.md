## Context

AegisCore 当前由 `common` 提供配置、日志、Redis/PostgreSQL helper 和中间件，由 `user-services` 在 `internal/bootstrap` 中组装 Fx 应用。现有配置根对象包含 `app`、`http`、`log`、`redis` 和 `postgres`，尚未提供跨服务系统级配置段，也没有统一初始化 `time.Local` 的机制。

参考实现 `go-micro-scaffold/common/pkg/timezone/module.go` 使用 Fx invoke、`sync.Once`、`time.LoadLocation`、`time.Local` 和 `TZ` 环境变量完成全局时区初始化。迁移到 AegisCore 时需要保留核心行为，但应避免 panic，改为返回启动错误以符合现有基础设施依赖失败即启动失败的契约。

## Goals / Non-Goals

**Goals:**

- 在 `common/config` 中增加 `SystemConfig`，通过 `system.timezone` 表达 IANA 时区名称。
- 在 `common` 中提供共享 timezone Fx module，由服务显式引入并在启动图中初始化全局本地时区。
- 默认时区为 `Asia/Shanghai`，保持当前参考实现的默认行为。
- 无效时区配置返回包含底层原因的错误，阻止服务继续启动。
- 用户服务接入该 module，并在默认 YAML 中声明 `system.timezone` 示例配置。
- 通过测试覆盖配置加载、timezone 初始化和用户服务 module wiring。

**Non-Goals:**

- 不改变 HTTP API、响应信封、错误码或路由。
- 不修改 Ent schema、生成代码或 Atlas migration。
- 不引入按请求、按用户或多租户时区切换。
- 不让 `common/config.Load` 承担 required/range 校验；它只负责读取、覆盖和反序列化。

## Decisions

1. 将配置字段设计为 `system.timezone`。

   该字段是跨服务系统级运行时设置，不属于 `app` 元数据、HTTP server、日志输出或 datastore 命名实例。新增 `SystemConfig` 可以为后续少量系统级选项留出清晰边界，同时避免污染现有配置段。

   备选方案是放入 `app.timezone` 或 `log.timezone`。`app.timezone` 容易与服务标识混合；`log.timezone` 只能解释日志日期，无法覆盖业务代码依赖 `time.Local` 的行为。

2. 将 timezone 能力放在 `common/timezone`，并提供 Fx `Module`。

   时区初始化是跨服务共享运行时能力，放在 `common` 符合仓库分层。包内提供 `Init` 或等价函数用于单元测试，Fx `Module` 通过 `fx.Invoke` 接入服务启动图。`common/infrastructure.Module` 继续只提供基础依赖，不强制所有服务初始化 timezone；具体服务通过 bootstrap 显式引入，保持 opt-in 装配风格与 Redis/PostgreSQL helper 一致。

   备选方案是直接在 `common/infrastructure.Module` 中 invoke timezone。该方案更省 wiring，但会让所有引入基础设施的工具或测试隐式修改全局进程时区，不利于控制副作用。

3. 初始化失败返回错误而不是 panic。

   参考实现使用 panic 终止进程。AegisCore 已经通过 Fx lifecycle 和 provider error 表达启动依赖失败，因此无效时区应返回错误，保留 `time.LoadLocation` 原始错误上下文，并由 Fx 阻止服务启动。

4. 保留 `sync.Once` 语义，但测试需要可控重置或隔离。

   全局时区和 `TZ` 是进程级状态，多次初始化可能导致不可预测结果。生产路径应只执行一次。测试可以通过包内未导出的 reset helper、子测试串行化、保存并恢复 `time.Local` 与 `TZ` 来避免污染其他测试。

5. 初始化顺序依赖 Fx 图而不是手写启动顺序。

   用户服务 module 通过引入 timezone module，使 Fx 在构造 HTTP server 和启动 lifecycle 前解析配置并执行 timezone invoke。测试应验证 module 可被装配，避免通过 HTTP handler 或 repository 层重复实现。

## Risks / Trade-offs

- [Risk] `time.Local` 和 `TZ` 是全局进程状态，测试之间可能互相影响。→ Mitigation: timezone 包测试串行执行，保存并恢复旧状态，并提供仅测试可用的 reset 路径。
- [Risk] `sync.Once` 会让运行中配置变更无法重新应用。→ Mitigation: 当前服务配置仅在启动时加载，运行期热更新不是本变更目标。
- [Risk] 无效时区会导致服务启动失败。→ Mitigation: 这是预期的 fail-fast 行为，错误信息保留配置值和底层加载错误。
- [Risk] 隐式引入到所有 common infrastructure 使用方可能影响迁移工具或纯配置测试。→ Mitigation: timezone module 由服务 bootstrap 显式引入，而不是放入 `common/infrastructure.Module`。
