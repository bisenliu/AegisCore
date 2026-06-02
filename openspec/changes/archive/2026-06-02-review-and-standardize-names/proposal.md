## Why

当前仓库已经形成多个稳定 capability，但目录、文件、函数、类型、规格与文档之间存在若干命名不一致或语义不够清晰的问题，尤其是 capability 映射与已存在规格不一致、规格中的响应码表达混用，以及部分内部 Go 命名过泛。现在进行一次受控的命名审查与标准化，可以提升代码可读性、维护性和后续变更的定位效率，同时避免借命名清理引入功能性或外部契约变更。

## What Changes

- 新增一项命名一致性治理 capability，用于约束项目内目录名、文件名、包名、函数名、类型名、OpenSpec capability 名与文档命名的一致性。
- 全面审查 `common`、`user-services`、`docs`、`openspec/specs` 与 `openspec/changes` 中的目录名、文件名、包名、主要函数名和类型名。
- 修正低风险且不改变外部行为的命名问题，例如内部变量/参数、内部类型、内部 helper 函数、文档与规格中的命名表达。
- 同步更新引用被改名对象的 Go import、测试、文档、OpenSpec 规格和 capability map。
- 明确保留已成为外部契约或迁移历史的一些名称，例如 `user-services` 模块路径、HTTP header、JSON 字段、响应码数值、已存在 migration 文件名和 Swagger 工具生成目录，除非另行提出 breaking change。
- 不引入新 API、业务行为、配置语义、数据库 schema 或迁移变更。

## Capabilities

### New Capabilities
- `project-naming-consistency`: 约束并验证项目命名审查、命名标准化、非功能性改名和引用同步的治理要求。

### Modified Capabilities
- `shared-infrastructure`: 仅在实现细节和文档表达层面统一共享基础设施相关命名，不改变配置、日志、Redis、PostgreSQL 或 Ent 运行时行为。
- `http-service-runtime`: 仅在内部运行时组装命名和文档表达层面统一命名，不改变 CLI、HTTP 路由、健康检查或优雅停机行为。
- `api-response-contract`: 仅统一规格和文档中的响应码命名表达，不改变响应信封字段、响应码数值或错误映射行为。
- `user-profile-query`: 仅在内部 controller/service/repository 命名和规格表达层面统一命名，不改变查询 API 行为。
- `database-schema-migrations`: 仅记录迁移文件命名约束，不重命名既有 migration 文件或修改 Atlas 校验历史。

## Impact

- 受影响代码：`common/` 和 `user-services/` 中与低风险内部命名相关的 Go 文件、测试文件和 imports。
- 受影响文档：`docs/opsx/CAPABILITY_MAP.md`、相关开发/架构说明，以及需要同步命名表达的 OpenSpec 主规格。
- API 兼容性：不改变 HTTP 路径、请求/响应 JSON 字段、响应码数值、header 名称、配置 key 或环境变量语义。
- 数据兼容性：不改变 Ent schema、Atlas migration、数据库表名或字段名；不重命名已存在 migration 文件。
- 依赖影响：不新增第三方依赖；若内部 Go 名称变更，需要同步更新引用并通过 `go test ./...` 验证。
