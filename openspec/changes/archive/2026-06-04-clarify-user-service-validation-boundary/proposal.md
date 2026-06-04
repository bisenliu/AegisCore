## Why

`user-services/internal/service/user_service.go` 和 `user-services/internal/service/auth_service.go` 当前在 Service 层执行 `strings.TrimSpace`、空值校验和部分基础参数解析，已经与项目约定的“HTTP 层处理请求解析、参数校验，Service 层负责业务编排和 DTO 映射”存在边界不一致。随着用户创建、列表、认证、会话和改密等接口的校验规则增加，继续把清洗和基础校验分散在 Service 方法中会增加重复逻辑、测试耦合和职责漂移风险。

## What Changes

- 明确请求输入清洗和基础参数校验的职责边界：Controller/Validation 层负责 HTTP 请求绑定、字段清洗、格式和必填校验；Service 层保留业务规则、业务编排、领域错误映射和 DTO 输出。
- 为用户服务引入服务内 `internal/validation` 层作为复杂请求校验的承载点，复用 `common/validation` 的通用能力，不把复杂规则直接堆叠到 Controller 或 Service。
- 将当前 `CreateUser`、`ListUsers`、`GetUserByID`、`Login`、`ChangePassword`、`Refresh` 中的基础输入处理作为迁移目标：字符串裁剪、必填字段校验、分页归一化、UUID 格式校验和请求体 token 空值处理优先在 Controller/Validation 边界完成，Service 接收已规范化输入。
- 保持外部 API 路径、响应信封和错误码兼容，不引入新的数据库模型或运行时依赖。

## Capabilities

### New Capabilities

- `user-service-validation-boundary`: 定义用户服务 Controller、Validation、Service、Repository 层之间的输入清洗、基础校验、业务校验和错误映射职责划分。

### Modified Capabilities

- `request-validation`: 扩展现有请求校验能力的适用边界，明确服务内复杂校验可通过 `internal/validation` 编排，Controller 不应承载大量复杂规则。
- `user-profile-create`: 创建用户请求的字符串清洗、必填校验和创建业务规则职责边界调整。
- `user-profile-query`: 查询用户详情和列表请求的 UUID、分页和过滤参数处理职责边界调整。
- `user-session-control`: 登录、改密和刷新请求的输入清洗/基础校验与认证会话业务规则职责边界调整。

## Impact

- 影响代码：`user-services/internal/controller/user_controller.go`、`user-services/internal/controller/auth_controller.go`、`user-services/internal/service/user_service.go`、`user-services/internal/service/auth_service.go`、拟新增或调整的 `user-services/internal/validation/`、相关 controller/service 测试。
- 影响能力：`request-validation`、`user-profile-create`、`user-profile-query`、`user-session-control`，并新增 `user-service-validation-boundary` 作为层级职责约束。
- API 兼容性：不改变现有 HTTP 路径、请求字段、响应信封、错误码语义或数据模型；校验失败仍通过 `common/response` 输出统一错误。
- 依赖影响：不引入新的外部服务、数据库迁移或 Ent schema 变更；优先复用 `common/validation` 和现有响应错误模型。
