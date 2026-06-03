## Context

本 change 面向代码评审反馈的结构化表达，不直接改变运行时代码。现有仓库已经存在 `project-naming-consistency` capability，用于约束命名审查、低风险重命名边界和引用同步；也存在 `shared-infrastructure` capability，用于约束 `common` 模块中的配置、日志、Redis、PostgreSQL、Ent client 和相关 Fx wiring。

本次输入包含两个评审主题：一是 `user-services/internal/errmsg/` 包名需要符合 Go 包命名最佳实践；二是 `common/infrastructure/` 随基础设施组件增加后需要更清晰的目录组织。为了让反馈可执行，后续实现应输出中文反馈文本，并对每个主题分别给出问题说明、原因分析和建议改法。

## Goals / Non-Goals

**Goals:**

- 明确评审反馈输出结构：每条反馈必须包含问题说明、原因分析和建议改法。
- 明确 Go 包命名反馈的判断依据：包名应短小、全小写、语义明确，缩写词不使用混合大小写风格。
- 明确共享基础设施目录组织反馈的判断依据：随着 Redis、PostgreSQL、MongoDB、RabbitMQ 等组件增加，应避免单目录职责混杂。
- 为后续实现提供可直接落地的文本组织方式和目录拆分建议。

**Non-Goals:**

- 不在本 change 中设计外部 HTTP API、错误码、响应信封、配置 key 或数据库 schema 变更。
- 不要求立即完成 `user-services/internal/errmsg/` 包重命名或 `common/infrastructure/` 目录重构。
- 不引入新的基础设施 client、Fx provider 或外部依赖。

## Decisions

- 采用“评审主题 -> 问题说明 -> 原因分析 -> 建议改法”的固定结构。
  该结构比普通段落更便于执行和追踪，能够把规范依据、潜在影响和落地建议分开表达。替代方案是保留原始短评，但原始短评缺少原因和可执行路径，不利于团队达成一致。

- 将包命名问题归入 `project-naming-consistency`。
  `errmsg` 这类 Go 包名属于内部命名一致性与 Go 规范实践，不涉及外部 API 或运行时行为。替代方案是把它归入某个业务 capability，但该问题本质上不改变用户资料、认证或响应契约能力。

- 将目录组织问题归入 `shared-infrastructure`。
  `common/infrastructure/` 是共享基础设施 provider 和 runtime dependency wiring 的聚合点，目录拆分建议应受共享基础设施边界约束。替代方案是新增独立 capability，但该问题是既有 capability 的可维护性补充，不需要引入新能力。

- 目录拆分建议保持原则性，不在规格中固定唯一目录形态。
  后续实现可以按基础设施类型拆分，例如 `redis/`、`postgres/`、`mongo/`、`rabbitmq/`，也可以按职责拆分，例如 `datastore/`、`messaging/`、`logging/`、`config/`。这样能保留演进空间，避免在未真正重构前过早锁定实现细节。

## Risks / Trade-offs

- [Risk] 反馈过度指向具体重构路径，可能被误解为必须立即大规模改目录。 -> Mitigation: 在反馈中明确“建议改法”与“后续重构”边界，说明当前重点是可维护性风险和组织原则。
- [Risk] Go 包命名示例与现有路径相同，容易造成表达混淆。 -> Mitigation: 反馈中应强调问题是避免 `errMsg`、`err_msg`、`errorMsg` 等混合或不符合 Go 习惯的包名，并明确推荐全小写短包名 `errmsg` 或更语义化的全小写名称。
- [Risk] 基础设施拆分可能影响现有公共 Go API。 -> Mitigation: 反馈只提出拆分建议；真正实施时必须结合 `project-naming-consistency` 的引用同步要求，保留外部配置契约和运行时行为。
