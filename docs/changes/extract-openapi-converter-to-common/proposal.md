# Extract OpenAPI converter to common

## What

将当前用户服务内的 OpenAPI 转换工具拆分为跨服务可复用能力：

- 在 `common` 中新增无业务语义的 OpenAPI 生成辅助包，负责 Swagger 2 JSON 到 OpenAPI 3 的转换、文档规范化、JSON/YAML 序列化和 Go embed 源码渲染。
- 保留或新增薄命令入口，只负责读取输入输出路径、解析 flags、把服务侧配置传给 `common` 包。
- 更新 `user-service/scripts/openapi-generate.sh`，让它继续负责用户服务源码扫描范围和输出目录，但调用共享转换能力。
- 将用户服务特定约定参数化，包括业务 API server URL、根路径健康检查、BearerAuth 名称和描述、Go embed package name、generated-by 注释。
- 为共享转换能力补充单元测试，覆盖转换、server/path server 规范化、安全方案注入和 Go embed 渲染。

本变更不迁移 OpenAPI 文档路由、不迁移 `swag init` 的服务源码扫描配置，也不把用户服务的 `/api/v1`、健康探针或认证描述作为 `common` 的硬编码默认业务语义。

## Why

后续其他服务也会使用相同的 OpenAPI 生成链路。当前实现位于 `user-service/internal/tools/openapi-convert/main.go`，其他服务无法直接复用，而且其中同时混合了两类职责：

- 通用职责：OpenAPI 格式转换、校验、输出 JSON/YAML、生成 Go embed 文件。
- 用户服务职责：`/api/v1` server、`/livez`/`/readyz`/`/startupz` 根路径覆盖、`BearerAuth` 描述和生成注释。

直接把整个 `main.go` 搬进 `common` 会把用户服务语义带进共享模块，违反 `common` 只能承载跨服务稳定、无业务语义基础能力的边界。拆分后，新服务可以复用转换库和命令入口，同时仍在各自脚本或配置中声明自己的路由扫描范围、server、认证方案和输出包名。

## Scope

包括：

- 新增 `common/http/openapi` 或同等清晰的共享包，用于 OpenAPI 文档转换与渲染。
- 将 `github.com/getkin/kin-openapi` 作为 `common` 的直接依赖。
- 将 `gopkg.in/yaml.v3` 调整为 `common` 的直接依赖，若共享包直接使用 YAML 序列化。
- 将用户服务当前转换逻辑中可复用的部分迁入共享包。
- 让服务特定默认值通过 options 或 CLI flags 输入。
- 更新 `user-service/internal/tools/openapi-convert/main.go` 为薄 wrapper，或改为调用 `common` 提供的共享命令入口。
- 更新 `user-service/scripts/openapi-generate.sh` 中的转换命令参数。
- 更新 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和 `AGENTS.md` 中与 OpenAPI 生成工具归属相关的描述。

不包括：

- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不改变 `swag init` 解析源码注解的机制。
- 不迁移 `user-service/internal/router/openapi.go` 或运行时 OpenAPI UI/JSON 路由。
- 不改变 `user-service/docs/openapi.json`、`openapi.yaml` 和 `openapi.go` 的语义输出，除非生成注释只因参数化而保持等价更新。
- 不为未来服务提前创建空目录、空脚本或占位业务代码。
- 不改变业务 API、Gin route、认证授权逻辑、Ent schema、migration 或部署资产。

## Acceptance Criteria

- 共享 OpenAPI 转换能力位于 `common`，且不包含用户服务硬编码业务语义。
- 用户服务仍可通过 `make openapi-generate` 生成 `user-service/docs/openapi.json`、`openapi.yaml` 和 `openapi.go`。
- 生成后的用户服务 OpenAPI 3 文档仍使用 OpenAPI `3.0.3`，业务接口 server 仍为 `/api/v1`，健康探针路径仍可覆盖为 `/`。
- `BearerAuth` 安全方案仍出现在用户服务 OpenAPI 3 components 中，描述由用户服务脚本或 wrapper 参数提供。
- `common` 中新增测试覆盖共享转换行为。
- `user-service` 中薄 wrapper 或脚本测试/验证能证明服务侧参数正确传入。
- `make openapi-generate` 成功。
- 与改动范围匹配的 `go test` 和 `make architecture-lint` 通过。
