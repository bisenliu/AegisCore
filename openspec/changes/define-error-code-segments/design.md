## Context

`common/contract/errors` 当前提供 `Kind`、`Reason`、`Code` 和 `Message` 的应用错误契约，`common/http/response` 根据 `Kind` 推导 HTTP status。现有 `KindConflict` 已映射为 `409 Conflict`，`CodeConflict=40000` 的运行时链路和测试覆盖均存在，本次 change 不修复现有行为 bug，而是补齐错误码段分配和扩展规则。

错误码是公开响应 envelope 的一部分，属于跨服务共享契约。随着后续认证扩展、授权策略、业务冲突、资源查找、限流和配额类能力增加，如果没有明确段位和准入规则，开发者可能按 feature 或临时需求随意新增 code，导致客户端语义混乱、跨服务 code 冲突，或新增 `Kind` 后遗漏 HTTP 映射。

## Goals / Non-Goals

**Goals:**

- 在 `shared-platform-primitives` 规格中定义错误码段分配、预留范围和新增错误码准入规则。
- 在 `common/contract/errors/code.go` 中补充与规格一致的集中式注释，作为开发者新增 code 时的最近入口说明。
- 保持 `Code` 与 HTTP status 解耦，继续由 `Kind` 推导 HTTP status。
- 明确新增 `Kind` 时必须同步 `common/http/response.statusCode` 和响应测试。
- 不保留任何兼容分支，因为本次不改变现有行为、数值或公开响应结构。

**Non-Goals:**

- 不新增 rate limiting、quota、OAuth、MFA 之外的新业务错误码。
- 不改变现有错误码数值，包括 `CodeConflict=40000`、`CodeNotFound=50000` 和 `CodePasswordChangeRequired=20006`。
- 不改变 response envelope、HTTP status 映射、OpenAPI 输出、数据库 schema、部署清单或观测资产。
- 不在 user-service feature、HTTP transport 或共享根包中新增 feature 专用错误映射表。

## Decisions

1. 使用现有 `shared-platform-primitives` capability 承载规则。

   理由：错误码位于 `common/contract`，是跨服务共享契约的一部分，能力地图已将该路径归入 `shared-platform-primitives`。无需新增单独 capability，避免把一次契约治理拆成过细规格。

   备选方案：新增 `contract-response-envelope` capability。暂不采用，因为能力地图将它列为未来拆分候选，仅在响应 envelope 或错误码契约成为独立变更热点时再拆分。

2. 使用 ADDED Requirement，而不是 MODIFIED 现有响应映射 Requirement。

   理由：现有“响应、错误归一化与 HTTP 映射”要求已经正确描述现有渲染行为，本次新增的是错误码段治理约束，不需要改写既有 HTTP 映射要求。

   备选方案：修改现有 Requirement 并嵌入段位规则。暂不采用，因为会扩大 delta 内容，增加归档时覆盖无关内容的风险。

3. 只添加注释和规格，不增加运行时校验或兼容逻辑。

   理由：当前错误码是 Go 常量，编译期和测试已经固定现有值。新增运行时校验无法覆盖“开发者未来如何占号”的主要风险，反而会引入与业务无关的生产逻辑。

   备选方案：新增 `ValidateCodeSegment` 或注册表。暂不采用，因为没有运行时输入需要校验，且会增加不必要 API 面和维护成本。

4. 将 `60xxx` 作为限流和配额预留段，但不启用具体 code。

   理由：用户明确提到未来可能增加 rate limiting 和资源配额。预留段能避免它们被误放入 `40xxx/50xxx`，同时不提前承诺未实现能力。

   备选方案：立即新增 `CodeRateLimited` 和 `CodeQuotaExceeded`。暂不采用，因为当前没有真实行为、`Kind`、HTTP 映射和测试需求。

## Risks / Trade-offs

- 错误码规则只靠注释和规格约束，不能自动阻止错误占号 -> 通过 `make user-service-architecture-lint`、代码审查和后续新增 code 时同步测试来约束。
- 预留 `60xxx` 可能让后续需求误以为限流和配额已经实现 -> 注释和规格明确该段启用前必须先定义 `Kind`、HTTP 映射和测试。
- 不拆分独立 capability 可能让错误码契约继续位于较大的共享平台原语规格中 -> 当前变更范围小，先保持现有能力地图；若响应契约频繁演进，再按能力地图候选拆分。
- 只补充说明不改变实现，无法提升现有测试覆盖范围 -> 当前 `CodeConflict` 数值和 409 映射已有测试；本次验证重点放在架构 lint 和相关包测试。

## Migration Plan

- 添加 OpenSpec delta 后，在实现阶段补充 `common/contract/errors/code.go` 注释。
- 不需要数据迁移、配置迁移、API 迁移、OpenAPI 重新生成或部署顺序调整。
- 回滚方式为移除本次新增规格 delta 和 `code.go` 注释；不会影响运行时行为。

## Open Questions

- 无。
