## Why

认证 command use case 当前通过 `UseCaseDeps` 共享依赖容器获取凭证、token、会话、指标和配置。该模式在当前规模下可运行，但没有把每个 use case 的最小依赖边界表达为编译期约束，后续多人协作时容易让实现访问并滥用不属于自身职责的依赖。

本次变更要在实现层收窄认证 command use case 的依赖面，使登录、刷新、改密、退出当前会话和退出全部会话只持有各自所需的 collaborator，并移除旧公共容器。

## What Changes

- **BREAKING** 移除 `auth/application/command` 包内的 `UseCaseDeps` 公共依赖容器和 `NewUseCaseDeps` 装配入口，不保留旧 constructor 兼容层。
- 为每个 auth command use case 定义独立的最小 Fx 输入参数结构，并在结构体字段中只保存该 use case 真实需要的依赖。
- 将原先依附在 `UseCaseDeps` 上的通用逻辑改为包内窄 helper 或显式依赖函数，避免 helper 迫使 use case 继续持有大容器。
- 更新 auth feature Fx provider 和 command 测试 fixture，使装配和测试直接使用新的最小依赖 constructor。
- 保持认证 HTTP API、OpenAPI 响应、数据库 schema、Redis key 语义和运行时配置项不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 收窄认证 command use case 的应用层依赖边界，要求 use case 只注入自身职责所需的最小 collaborator，避免共享依赖容器暴露无关依赖。

## Impact

- 影响代码：`user-service/internal/features/auth/application/command/`、`user-service/internal/features/auth/fx.go`、认证 command 相关测试。
- 不影响公开 HTTP 路由、请求/响应 DTO、OpenAPI 生成物、Ent schema、Atlas migration、部署资产或外部服务契约。
- 不新增第三方依赖。
- 实现后需要运行认证 command 相关测试，并运行 `make user-service-architecture-lint` 验证结构边界。
