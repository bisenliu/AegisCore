## MODIFIED Requirements

### Requirement: HTTP runtime naming cleanup preserves service contract
HTTP 服务运行时相关命名标准化 SHALL 只修改内部组装名称、局部变量、内部类型或文档表达，不得改变 CLI、路由注册、中间件顺序、健康检查、Swagger 暴露或优雅关闭行为。用户服务 Fx 运行时组装模块的 Go 符号名称 MUST 明确表达进程级运行时装配职责，并 MUST NOT 使用容易与 `internal/service` 业务 service 层混淆的名称。

#### Scenario: Runtime module names are standardized
- **WHEN** 实现修改 `user-services/internal/bootstrap`、`user-services/internal/router` 或 `user-services/cmd` 中的内部命名
- **THEN** 服务启动命令、HTTP 路由、Fx 依赖关系和关闭流程 MUST 与修改前保持等价

#### Scenario: Fx app module name reflects composition root scope
- **Given** 维护者查看 `user-services/internal/bootstrap/app.go` 中的用户服务 Fx 模块定义
- **When** 该模块装配 timezone、validation、PostgreSQL、Redis、JWT、Ent、repository、service、controller、Gin engine、HTTP server 和路由注册
- **Then** Go 符号名称 MUST 使用 `AppModule` 表达应用组合根职责
- **Then** Go 符号名称 MUST NOT 使用 `UserServiceModule`

#### Scenario: Service identity name is reviewed
- **WHEN** 审查发现 `user-services` 复数命名或服务名语义可改进
- **THEN** 本变更 MUST 保留该名称，并将目录、module path、CLI 名或服务标识重命名视为单独 breaking change
