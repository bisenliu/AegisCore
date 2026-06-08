## Context

AegisCore 是 Go backend workspace，`common/` 承载共享运行时、HTTP、响应、安全和校验能力，`user-services/` 承载用户服务 controller/service/repository、Ent schema、路由和运行时装配。现有 `project-naming-consistency` 已要求命名审查、低风险重命名边界、引用同步和结果报告，并已包含 `UserID` 相关 controller handler 命名要求。

本变更是跨模块、非功能性命名治理：依据 Go Code Review Comments 和 Uber Go Style Guide，统一常见 initialism 在手写 Go 标识符、测试、godoc、文档和 OpenSpec 引用中的大小写。实现必须保留外部契约字符串，例如 `user_id`、`session_id`、`token_version`、`X-Trace-ID`、JSON tag、配置 key、环境变量、数据库字段和迁移历史。

## Goals / Non-Goals

**Goals:**

- 建立 repository-wide initialism 规范，覆盖 `ID`、`API`、`HTTP`、`URL`、`JSON`、`UUID`、`JWT`、`TTL`、`SQL` 等常见缩写词。
- 审查并修正低风险内部 Go 标识符及其 workspace 内引用。
- 同步 godoc 注释、测试名称、文档、OpenSpec 规格和 capability map 中的内部符号引用。
- 对保留的高风险或受保护名称输出原因，确保实现结果可审计。

**Non-Goals:**

- 不改变 HTTP 路径、请求参数、JSON 字段、header、响应码、公开错误消息、配置 key、环境变量、Redis key 或数据库 schema。
- 不手写修改 `user-services/ent/` 下的生成代码。
- 不重命名 Atlas migration 文件、不修改 `atlas.sum`、不改写迁移历史。
- 不进行 controller/service/repository 分层重构、业务逻辑重构或新增业务能力。
- 不把本次治理扩大为所有 lint、注释覆盖率或包结构问题的全面整改。

## Decisions

### Decision: 以 `project-naming-consistency` 承载 initialism 治理

本变更修改现有 `project-naming-consistency` capability，而不是新增独立命名 capability。该 capability 已覆盖命名审查、重命名边界、引用同步和结果报告，initialism 规范是其自然延伸。

备选方案是新增 `go-style-governance` 或修改 `go-lint-automation`。新增 capability 会与现有命名治理重叠；`go-lint-automation` 更适合自动化工具和 CI 规则，不适合作为人工审查与保护边界的主规格。

### Decision: 先分类，再执行低风险 rename

实现阶段应先搜索非规范 spelling，例如 `UserId`、`Api`、`Http`、`Url`、`Json`、`Uuid`、`Jwt`、`Ttl`、`Sql`，并按外部契约、公共 Go API、内部 Go API、文档/规格表达、工具链、生成代码、迁移历史分类。只有低风险内部 Go API 和内部文档引用应直接修改。

这样可以避免把外部 contract 字符串误改为 Go 标识符风格。例如 `GetByUserId` 应改为 `GetByUserID`，但 route 参数 `user_id`、JSON tag `user_id` 和 header `X-Trace-ID` 必须保留。

### Decision: initialism 作为完整词保持一致大小写

导出或中间单词中的 initialism 使用全大写，例如 `UserID`、`APIClient`、`HTTPServer`、`JSONBody`、`UUIDValue`、`JWTToken`、`TTL`、`SQLDB`。未导出标识符仍保持 initialism 词整体小写或全大写组合后的 Go 风格，例如 `userID`、`apiClient`、`httpServer`、`jsonBody`、`uuidValue`、`jwtToken`、`ttl`、`sqlDB`。

不接受 `UserId`、`ApiClient`、`HttpServer`、`JsonBody`、`UuidValue`、`JwtToken`、`Ttl`、`SqlDB` 等混合大小写。包名仍遵循 Go 包名规则：短小、全小写、无下划线，且不因 initialism 规则引入大写包名。

### Decision: 注释与规格只同步内部符号，不改外部字段表达

godoc 注释必须与导出标识符同步，例如重命名为 `GetByUserID` 后注释应以 `GetByUserID ...` 开头。文档和 OpenSpec 中引用内部 Go 符号时也应使用新名称。

Swagger 描述、API 文档和规格中描述外部字段时仍使用真实 contract spelling，例如 `user_id`、`session_id`、`token_version`。这样既满足 Go 注释规范，又避免对外部 API 语义产生误导。

## Risks / Trade-offs

- 误改外部 contract 字符串 -> 通过候选分类和最终复查保护 HTTP path、JSON tag、header、配置、Redis key、DB 字段和迁移历史。
- 公共 Go API rename 影响潜在外部消费者 -> 默认只修改 workspace 内低风险内部符号；如发现对外发布 API，应报告并拆分为单独兼容性变更。
- 生成代码与 schema 来源混淆 -> 不手写修改 `user-services/ent/` 生成代码；如必须改 Ent schema，另行执行 `go generate ./ent` 和 migration 流程，但本变更默认不涉及 schema。
- 文档批量替换造成语义漂移 -> 只同步内部符号引用，不把外部字段名从 snake_case 改为 Go 标识符风格。
- 命名审查范围过大导致无关 lint cleanup 混入 -> tasks 限定为 initialism 与明显相关的 godoc/引用一致性，不处理无关格式或业务重构。
