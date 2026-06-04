## Context

当前 `user-services/internal/service/user_service.go` 在 `CreateUser` 中执行 `strings.TrimSpace` 和空值校验，在 `ListUsers` 中裁剪过滤字段，在 `GetUserByID` 中解析 UUID。`user-services/internal/service/auth_service.go` 也在 `Login` 中裁剪用户名和密码，在 `authenticateUser` 中校验空凭据，在 `ChangePassword` 中裁剪并校验新密码，在 `Refresh` 中剥离 Bearer 前缀并校验空 Refresh Token。项目开发约定要求 HTTP 层处理请求解析和参数校验，Service 层负责业务编排和 DTO 映射，Repository 层负责数据库访问和存储错误转换。

这些逻辑目前规模较小，但已经混合了三类责任：

- 输入规范化：裁剪 `nickname`、`username`、`password`、登录密码、新密码、过滤字段，剥离请求体 token 的可选 Bearer 前缀等原始请求值。
- 基础参数校验：必填、UUID 格式、分页默认值、边界归一化、请求体 token 空值校验。
- 业务规则：用户名唯一性、密码哈希、默认状态、用户 ID 生成、凭据认证、token claims 校验、session 校验、领域错误映射。

当后续新增用户更新、认证、权限、查询条件组合、密码强度、字段级错误明细等复杂校验时，如果继续放在 Service 层，会让 Service 既像应用服务又像请求验证器，导致复用困难、测试维度混杂和层级边界不稳定。

## Goals / Non-Goals

**Goals:**

- 明确 `strings.TrimSpace`、必填校验、UUID 格式校验、分页归一化、请求体 token 规范化等基础输入处理是否属于 Service 职责。
- 为后续大量复杂校验提供可扩展的位置，避免 Controller 和 Service 同时膨胀。
- 保持现有 HTTP API、响应信封、错误码和数据库模型兼容。
- 让 Service 方法接收已规范化、已完成基础校验的请求对象，专注业务编排、领域规则和 DTO 输出。

**Non-Goals:**

- 不新增外部校验依赖。
- 不改变 `common/response.Envelope`、错误码语义或路由。
- 不修改 Ent schema、不生成数据库 migration。
- 不把所有业务规则都迁移出 Service；需要 Repository 查询或领域状态判断的规则仍由 Service 编排。

## Decisions

### 1. 引入服务内 Validation 层，而不是只放 Controller 或继续放 Service

推荐在 `user-services/internal/validation/` 增加服务内校验组件，承载用户服务请求清洗和基础校验。Controller 负责 Gin 绑定、路径和查询参数提取、调用 validation、输出响应；Validation 层返回规范化 DTO 或错误；Service 接收规范化输入并执行业务编排。

备选方案：全部放 Controller。优点是边界最靠近 HTTP 输入，短期文件更少；缺点是 Controller 会快速积累字段规则、组合规则、清洗规则和错误明细映射，后续接口增多时可读性和测试性下降。

备选方案：继续放 Service。优点是改动最少，Service 可防御非 HTTP 调用者传入脏数据；缺点是职责越界更明显，业务测试必须覆盖输入清洗细节，且 Controller/Swagger/common validation 的字段语义容易与 Service 内部校验分叉。

推荐方案的取舍：新增一层会增加少量文件和依赖注入成本，但能把后续复杂校验集中管理，符合当前 `request-validation` capability 已存在且未来校验复杂度会上升的趋势。

### 2. Validation 层负责“请求级”清洗和基础校验，Service 层负责“业务级”规则

Validation 层职责：

- 对原始 DTO 字符串执行 `strings.TrimSpace`。
- 校验必填、长度、格式、枚举、UUID、分页和查询过滤字段。
- 对登录、改密和刷新请求执行请求体字段规范化，例如裁剪 username/password/new_password，并对请求体中的 refresh token 或 password-change token 剥离可选 Bearer 前缀后校验非空。
- 对分页参数调用 `response.NormalizePagination` 或等价封装，输出确定的 page/pageSize/offset/limit。
- 返回 `response.ValidationFailedError` 或 `response.BadRequestError` 等现有统一错误，不改变响应契约。
- 在复杂接口中集中处理跨字段请求规则，例如查询条件组合、密码强度、字段互斥等不依赖数据库状态的规则。

Service 层职责：

- 使用已规范化输入执行应用业务流程。
- 查询 Repository 判断用户名唯一性、用户是否存在、状态是否允许操作等依赖持久化状态的业务规则。
- 生成用户 ID、计算密码哈希、设置业务默认值、验证登录凭据、解析和校验 JWT claims、校验 Redis session/token version、编排 Repository 调用。
- 将领域错误映射为统一响应错误，并输出业务日志。

Controller 层职责：

- 解析 Gin path/query/body 参数。
- 调用 `common/validation` 或服务内 `internal/validation`。
- 调用 Service 并通过 `common/response` 输出 HTTP 响应。
- 不承载大量字段级和组合规则，只负责请求边界编排。

Repository 层职责保持不变：只处理 Ent/数据库访问和存储错误转换，不处理 HTTP 请求校验。

### 3. 当前 Service 中的 `TrimSpace` 和空值校验属于轻度职责越界，应迁移但不需要激进重构

`CreateUser` 中的 `nickname`、`username`、`password` 裁剪和空值校验是请求级输入清洗与基础参数校验，应下沉到 Validation 层或由 Controller 边界完成。`ListUsers` 中查询过滤字段裁剪和分页归一化同样属于请求级规范化。`GetUserByID` 的 UUID 解析更接近路径参数格式校验，应在 Controller/Validation 层完成，并可让 Service 接收 `uuid.UUID` 或已校验的字符串。

`auth_service.go` 中 `Login` 的 username/password 裁剪、空凭据拦截，`ChangePassword` 的新密码裁剪和空值校验，以及 `Refresh` 对请求体 refresh token 的 Bearer 前缀剥离和空 token 校验，也属于请求级清洗或基础校验，应迁移到 Controller/Validation 边界。`verifyPasswordChangeToken` 内解析 password-change token、校验 token version、解析 claims 中 user_id，`Refresh` 内校验 refresh subject、claims user_id、Redis session 和 token version，`authenticatedSession` 从认证上下文读取并校验 user_id/session_id，则属于认证会话业务语义或安全边界，短期应保留在 Service 或认证中间件相关边界，不应简单下沉到普通请求 Validation 层。

Service 可保留少量防御性假设，例如遇到 Repository 返回领域错误时仍做响应错误映射，但不应把基础输入校验作为主要职责。迁移时可以逐接口进行，避免一次性大改影响稳定性。

### 4. 保持 `common/validation` 为通用基础，`internal/validation` 为服务规则

`common/validation` 应继续提供跨服务通用能力，例如 Gin 绑定、结构体 tag 校验、字段明细格式化和校验失败响应。用户服务特定的规则不应放入 `common`，避免共享模块被业务细节污染。

`user-services/internal/validation` 应依赖通用能力并封装服务内规则，例如 `ValidateCreateUser`、`ValidateListUsers`、`ValidateUserID`、`ValidateLogin`、`ValidateChangePassword`、`ValidateRefreshTokenRequest`。如果未来多个服务重复出现同类通用校验，再考虑抽象回 `common/validation`。

## Risks / Trade-offs

- [风险] 新增 Validation 层可能让简单接口显得层级过多 → [缓解] 只对有清洗、多个字段或复杂规则的接口引入函数；简单只读接口可保持轻量边界。
- [风险] Controller、Validation、Service 之间 DTO 命名和数据流变复杂 → [缓解] 优先复用现有 DTO，并仅在必要时新增 normalized input 类型，避免过度建模。
- [风险] Service 不再校验基础输入后，非 HTTP 调用者可能传入未规范化数据 → [缓解] 将 Service 接口约定更新为接收已验证输入；如存在真实非 HTTP 调用方，再为该调用边界复用同一 Validation 层。
- [风险] 校验错误消息或错误码在迁移中发生变化 → [缓解] 迁移前后保持 `errmsg` 和 `common/response` 语义一致，并增加 controller/validation 测试覆盖。
- [权衡] 相比 Service-only，推荐方案增加少量代码结构；换来职责清晰、复杂校验可维护、测试粒度更稳定和 API 文档语义更一致。

## Migration Plan

1. 在 `user-services/internal/validation/` 增加用户资料与认证请求校验函数，复用现有 `errmsg`、`common/auth`、`common/response` 错误模型。
2. 调整 `user_controller.go` 和 `auth_controller.go` 在调用 Service 前完成 path/query/body 参数规范化和基础校验。
3. 调整 `user_service.go` 移除请求级 `strings.TrimSpace`、空值校验、UUID 解析和分页归一化，仅保留业务编排和业务规则。
4. 调整 `auth_service.go` 移除登录/改密/刷新请求级裁剪和空值校验，保留凭据认证、JWT claims 校验、session/token version 校验、密码哈希和业务错误映射。
5. 更新或新增测试，分别覆盖 Validation 层的清洗/基础校验、Controller 的响应输出和 Service 的业务流程。
6. 运行 `go test ./...` 于受影响的 `user-services/` 模块。

回滚策略：若迁移导致行为异常，可将 Controller 调用 Validation 的改动回退，同时恢复 Service 中原有基础校验；不涉及数据模型或 migration，回滚不需要数据库操作。

## Open Questions

- Service 接口是否应从字符串 `userID` 调整为 `uuid.UUID`，以通过类型系统表达“已校验”的路径参数。推荐在实现时评估测试和调用点影响，若调用点仅 Controller 一个，则优先改为 `uuid.UUID`。
- 分页归一化是否保持使用 `common/response.NormalizePagination`，还是在 `common/validation` 中提供更语义化的分页输入校验 helper。推荐短期保持现有 helper，避免扩大变更范围。
- Auth Service 接口是否应接收已剥离 Bearer 前缀的 refresh/password-change token。推荐只对请求体 token 做边界规范化；JWT claims 与认证上下文校验继续留在 Auth Service 或认证中间件边界。
