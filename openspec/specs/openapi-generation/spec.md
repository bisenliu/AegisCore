# OpenAPI 生成规格

## 需求

### 需求：服务拥有生成语义
user-service 脚本必须拥有 `swag init` 扫描范围、API server URL、认证方案描述、输出目录和生成包名。

#### 场景：生成 OpenAPI
Given API 注解发生变化
When 执行 `make user-service-openapi-generate` 或模块级 `make -C user-service openapi-generate`
Then `user-service/docs/` 下的 OpenAPI 生成物必须通过服务脚本更新。

### 需求：公共 helper 保持通用
`common/http/openapi` 只能拥有 Swagger/OpenAPI 转换、规范化、序列化和 Go embed 渲染能力。不得拥有服务 API server 元数据、健康路径、认证方案描述、源码扫描范围或输出目录。

#### 场景：新增服务特定 OpenAPI 元数据
Given 元数据描述 user-service API 行为
When 选择代码位置
Then 必须放在 user-service 脚本或薄 wrapper 中，不得放入 common helper。

### 需求：运行时文档路由归属
运行时 OpenAPI UI、JSON 和 docs redirect 必须由 `user-service/internal/router/openapi.go` 拥有。

#### 场景：健康或指标路由
Given 系统存在健康检查或指标 endpoint
When 生成功能 API 文档
Then 不得把它们当作 `/api/v1` 下的功能业务 API。
