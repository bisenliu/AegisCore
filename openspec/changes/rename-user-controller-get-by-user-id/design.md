## Context

用户查询 API 当前通过 `GET /api/v1/users/:user_id` 查询外部 UUID 用户 ID。controller 方法名为 `GetByID`，但请求路径、Swagger 注解、DTO、service 和 repository 查询语义都围绕外部 `user_id` 展开。项目中同时存在内部数据库自增 `id int64` 和外部 UUID `user_id`，因此 controller handler 的泛化命名容易让维护者误解查询键类型。

该变更属于低风险内部 Go 符号命名标准化，相关能力为 `project-naming-consistency` 和 `user-profile-query`。实现不得改变 HTTP API、响应契约、认证要求、数据库 schema、Ent 生成代码或 Atlas migration。

## Goals / Non-Goals

**Goals:**

- 将 `UserController.GetByID` 重命名为 `UserController.GetByUserID`，使 controller handler 与路由参数 `user_id` 和 service 方法 `GetUserByID` 保持语义一致。
- 同步更新 `user-services/internal/router/users.go` 中的 handler 引用。
- 同步更新相关 OpenSpec 规格对内部 handler 名称和路由绑定的引用，避免主规格继续固定旧名称。
- 验证用户服务模块编译和测试通过。

**Non-Goals:**

- 不新增按内部自增 ID 查询用户的 API 或后台能力。
- 不修改 `GET /api/v1/users/:user_id` 路径、Swagger path 参数名或响应 JSON 字段。
- 不修改 service/repository 方法名、Repository 抽象、Ent schema、migration 或数据库查询条件。
- 不调整校验、认证、错误映射或响应信封实现。

## Decisions

1. Controller handler 使用 `GetByUserID`。

   理由：`GetByUserID` 与现有 service 方法 `GetUserByID` 和 repository 方法 `GetByUserID` 的外部用户 ID 语义一致，同时保留 controller handler 的动作式命名风格。相比 `GetUserByID`，`GetByUserID` 更贴近当前 `UserController` receiver 已经提供的用户上下文，避免 `UserController.GetUserByID` 出现轻微重复。

   替代方案：使用 `GetUserByID`。该名称也正确，但在 `UserController` 上读作 `UserController.GetUserByID`，上下文略有重复。当前目标是最小命名修正，因此选择只补充查询键语义。

2. 只重命名 controller handler 和直接引用点。

   理由：service/repository 已经使用 `GetUserByID`、`GetByUserID` 表达外部用户 ID，无需扩大改动范围。DTO、path 参数和 Swagger 注解已使用 `user_id`，也不需要修改。

   替代方案：统一重命名整条调用链。该方案会扩大无必要的非功能性改动，并增加回归风险。

3. 将规格变更限定为内部命名与兼容性要求。

   理由：这是内部 Go API 命名标准化，不应改变用户资料查询能力的外部行为。规格 delta 应明确 handler 新名称和兼容性边界，而不是引入新业务能力。

## Risks / Trade-offs

- [Risk] 只改 controller 方法名可能遗漏路由引用，导致编译失败。→ 通过同步更新 `user-services/internal/router/users.go` 并运行 `go test ./...` 验证。
- [Risk] Swagger 生成工具可能依赖 godoc 注释函数名。→ 同步将注释前缀从 `GetByID godoc` 改为 `GetByUserID godoc`，保持注解内容不变。
- [Risk] 规格或文档仍残留旧的 `UserController.GetByID` 引用。→ 搜索 workspace 中的 `GetByID` 和 `UserController.GetByID`，同步必要引用。
- [Risk] 非功能性重命名被误解为 API 变更。→ tasks 和规格明确禁止修改 HTTP 路径、响应 envelope、错误码、数据库 schema 和 migration。
