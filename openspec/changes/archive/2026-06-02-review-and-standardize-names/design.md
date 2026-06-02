## Context

当前仓库是 Go workspace，包含 `common` 和 `user-services` 两个模块。`common` 承载共享配置、日志、中间件、响应契约和基础设施 provider；`user-services` 承载用户服务 HTTP runtime、controller/service/repository 分层、Ent schema、Atlas migration 和 Swagger 文档。现有 capability 已通过 `docs/opsx/CAPABILITY_MAP.md` 与 `openspec/specs/*/spec.md` 表达，但实际仓库中存在 capability map 滞后、规格表达不统一、部分内部 Go 命名过泛等问题。

本变更是跨模块的非功能性命名治理。实现必须先审查命名，再只修改低风险命名和文档/规格表达；对已成为外部契约、工具链约定或迁移历史的名称必须保留，除非后续单独提出 breaking change。

## Goals / Non-Goals

**Goals:**

- 建立 `project-naming-consistency` capability，明确项目命名审查和标准化的长期要求。
- 全面审查目录名、文件名、Go 包名、类型名、函数名、方法名、OpenSpec capability 名和文档命名。
- 修复不改变行为的低风险命名问题，并同步更新引用、测试、文档和规格。
- 修正 `docs/opsx/CAPABILITY_MAP.md` 与现有 `openspec/specs` 的 capability 状态不一致问题。
- 统一规格和文档中的响应码、trace-id 边界名、内部 helper 命名等表达方式。
- 通过 Go 测试验证命名变更没有破坏编译和行为。

**Non-Goals:**

- 不重命名 `user-services` 目录、Go module path、CLI 名、服务名或 Swagger import path。
- 不修改 HTTP 路由、请求/响应 JSON 字段、响应 `code` 数值、`X-Trace-ID` header、日志字段、配置 key 或环境变量语义。
- 不修改 Ent schema、数据库表字段、Atlas migration 内容或已存在 migration 文件名。
- 不新增业务能力、外部 API、认证策略、数据模型或第三方依赖。
- 不手写 `user-services/ent/` 下的生成代码。

## Decisions

1. 采用“先分类、再改名”的审查策略。

   命名项按风险分为外部契约、公共 Go API、内部 Go API、文档/规格表达、工具链/迁移历史五类。外部契约和迁移历史默认不改；公共 Go API 只有在收益明确且调用面可控时才改；内部 Go API 和文档/规格表达优先清理。

   备选方案是直接批量重命名所有看起来不一致的名称。该方案会增加 module path、Swagger、迁移、配置和 API 契约破坏风险，因此不采用。

2. 保留 `user-services` 目录和 module path。

   虽然单个服务使用复数名称语义不够精确，但该名称已经关联 `go.mod` module path、imports、运行命令、配置路径、Swagger import、文档路径和可能的部署约定。将其改为 `user-service` 属于 breaking change，不纳入本次非功能性清理。

   备选方案是在本次变更中同步重命名目录、module 和所有 imports。该方案需要额外迁移计划和兼容策略，超出命名 cleanup 的风险边界。

3. 对 response code 只统一规格表达，不改变 API 值。

   `api-response-contract` 中的响应 `code` 当前是数字枚举。规格中可以使用 Go 常量名或明确语义标签，但实现不得把外部 JSON `code` 改成字符串，也不得改变数值映射。

   备选方案是把响应码统一改为字符串码。该方案会改变客户端可观察行为，因此不采用。

4. 将 trace-id 的不同边界命名视为有意约定。

   HTTP header 使用 `X-Trace-ID`，Gin context key 使用 `trace_id`，日志字段使用 `trace-id`。实现可改进文档说明或低风险文件名，但不得改变这些边界名称。

   备选方案是把 header、context key 和日志字段全部统一为一种拼写。该方案会破坏 HTTP 和可观测性契约，因此不采用。

5. 优先处理内部 Go 命名清晰度。

   可优先改名的候选包括 `bootstrap.Module`、`entclient.Params`、`entclient.Clients`、controller 构造参数 `service`、service mapper `userResponse`、Fx provider 参数名等内部名称。改名后必须同步 imports、测试和文档引用。本次实现将这些内部名称分别标准化为 `UserServiceModule`、`ClientParams`、`NamedClients`、`userService`、`toUserResponse`、`fxName` 和 `configKey` 等更清晰的名称。

   备选方案是保留所有代码命名，只修正文档。该方案风险最低，但无法满足“函数名/文件名全面审查并直接修改”的目标。

6. 不重命名既有 migration 文件。

   已存在 migration 文件名参与迁移历史和校验语境。即使名称偏泛，也只在规范中要求未来 migration 使用更具体名称，不对既有文件重命名。

   备选方案是同步修改 migration 文件名和 `atlas.sum`。该方案可能破坏历史一致性，因此不采用。

## Risks / Trade-offs

- 内部 Go 改名遗漏引用导致编译失败 -> 使用 `go test ./...` 分别验证 `common` 和 `user-services` 模块。
- 文档/规格改名与真实代码行为不一致 -> 实现前以现有代码和主规格为事实来源，变更后复查 `docs/opsx/CAPABILITY_MAP.md` 与 `openspec/specs`。
- 公共 Go API 改名影响潜在外部调用方 -> 默认避免公共 API 改名，若必须改则只在当前 workspace 调用面可控时执行。
- 命名 cleanup 范围膨胀为功能重构 -> tasks 明确禁止 API、配置、schema、migration 和业务逻辑变更。
- Swagger 文档 schema 名改动影响外部文档契约 -> 默认不重命名 DTO 类型，除非确认仅内部使用且同步更新 OpenAPI 产物。

## Migration Plan

1. 扫描仓库命名并形成候选清单，按风险分类。
2. 修改低风险内部 Go 命名和文档/规格命名表达。
3. 同步更新引用、tests、OpenSpec 主规格和 capability map。
4. 运行 `gofmt` 与模块测试。
5. 若测试失败，修复引用或回退单个高风险命名项，不回退无关用户改动。

回滚策略：本变更不涉及数据迁移或外部 API；若某个改名引入兼容风险，可仅回滚该命名项及其引用，保留文档中对保留名称的说明。

## Open Questions

- 是否将公共 `common` API 的轻微命名问题纳入本次实现，取决于实际调用面和收益；默认只处理内部或文档/规格层面的名称。
- 是否重命名 Swagger DTO 类型取决于生成文档契约影响；默认避免改变 OpenAPI schema 名。
