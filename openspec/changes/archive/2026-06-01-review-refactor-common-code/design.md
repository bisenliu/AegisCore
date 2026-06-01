## Context

`common/` 是多个服务复用的共享层，当前覆盖 `common/config`、`common/infrastructure`、`common/logger`、`common/middleware`、`common/response`、`common/validation`。这些包已经支撑 `shared-infrastructure`、`api-response-contract` 和 `request-validation` 三个能力，但实现中仍存在三类问题：运行时策略硬编码、部分失败路径缺少边界保护、若干文件同时承载过多职责。

当前主规格明确 `common/config.Load` 只负责读取、环境变量覆盖和反序列化，不执行 required/range 校验。本设计不改变 `Load` 的职责，也不新增配置文件字段校验或默认值设置能力。

## Goals / Non-Goals

**Goals:**

- 在不破坏现有公开 API 的前提下，为共享中间件、响应、校验、日志和 datastore provider 增加可测试的安全边界。
- 保持 `common/config.Load` 的反序列化职责不变，不引入配置字段校验或默认值填充流程。
- 将 `validation.go`、`logger.go` 的内部实现拆为职责清晰的文件，降低后续修改 blast radius。
- 将重复硬编码值提取到所属包内常量或 options，避免创建跨包“大 constants 垃圾桶”。
- 通过 opt-in helper 简化 Redis/PostgreSQL 命名依赖的 Fx wiring，但继续避免自动连接未声明实例。

**Non-Goals:**

- 不新增认证、授权、支付、健康检查聚合或新的业务 API。
- 不修改数据库 schema，不生成 Atlas migration，不手写 `user-services/ent/` 生成代码。
- 不把所有 `common` 包合并为一个超大共享模块，也不引入新的外部运行时依赖。
- 不新增 `common/config` 的字段校验 API、默认值 API、启动前配置准备流程或配置文件 schema 校验。
- 不强制所有调用方立即启用严格 JSON unknown-field 拒绝；严格行为应通过 options 或后续服务显式采用。

## Decisions

1. `common/config` 保持反序列化边界，不承担字段校验或默认值设置。

   理由：当前项目已明确配置加载不做 required/range 校验，后续也不需要在共享配置层增加默认值或字段校验能力。Redis/PostgreSQL ping、close 和 Fx lifecycle 仍属于基础设施运行时职责，配置值是否可被底层依赖接受由运行时初始化或依赖库处理。

2. Middleware 采用 options 扩展，保留现有便捷函数。

   理由：`CORS()`、`TraceID()` 等现有函数可能已被服务使用，应继续作为默认 wrapper；新增 `CORSWithOptions`、`TraceIDWithOptions` 或等价 API 承载生产策略、测试策略和安全边界。

3. 常量保持在 owning package 内。

   理由：`X-Trace-ID`、`trace-id`、响应默认消息、validation tag/rule、CORS header/method 等值分别属于不同包的公共契约或实现细节。按包内聚可以减少 import 环和跨层耦合。替代方案是新增 `common/constants`，但它会弱化所有权边界。

4. `common/validation` 先做文件级拆分，再做行为加固。

   理由：该包同时包含 Fx module、validator 初始化、Gin binding、反射 binding、错误归一化、字段名解析、翻译和 `BindOrAbort` 响应/日志集成。先拆分为 `module.go`、`validator.go`、`binder.go`、`errors.go`、`fields.go`、`translations.go` 等文件，可以降低行为变更时的风险。

5. `common/logger` 拆分时同时处理默认 logger 并发安全。

   理由：当前全局默认 logger 是共享状态，测试或启动流程中并发读写可能产生 race。拆分为 context helper、factory、writer、daily writer、level helper 后，可以用 `sync.RWMutex`、`atomic.Value` 或等价机制隔离默认 logger 访问。

6. 命名 Redis/PostgreSQL provider helper 必须显式 opt-in。

   理由：`shared-infrastructure` 已规定不得因为配置中存在实例而自动连接全部 Redis/PostgreSQL。helper 只减少服务侧重复 `fx.Provide` wiring，不能改变“声明哪个才连接哪个”的运行时语义。

## Risks / Trade-offs

- 配置字段校验或默认值设置重新进入范围会改变既有约束 -> 本 change 明确不实现相关 API，也不要求服务启动流程接入配置准备步骤。
- CORS/trace-id 默认行为过度收紧可能影响现有调用方 -> 保留现有默认 wrapper，并将更严格策略放在 options 中逐步采用。
- 文件拆分容易引入无行为变更的回归 -> 拆分提交应配合现有测试，优先移动代码和补齐测试，再做行为增强。
- JSON binding 严格 unknown-field 拒绝可能破坏兼容客户端 -> trailing JSON 拒绝作为安全默认，unknown-field 拒绝通过 opt-in option 或后续服务明确开启。
- 全局 logger 并发安全实现可能影响测试隔离 -> 提供可恢复默认 logger 的测试辅助或在测试中显式 `SetDefault` 并清理。
- Datastore helper 可能被误解为自动连接所有配置实例 -> API 命名和文档必须强调单实例、显式声明、按名称 opt-in。
