## Why

仓库已有 `UserID`、`HTTP`、`API`、`UUID`、`JWT` 等 Go 风格命名要求，但仍需要一次面向 Go Code Review Comments 和 Uber Go Style Guide 的全局 initialism 审查，避免 `UserId`、`Http`、`Api` 等混合大小写在代码、注释和文档中继续扩散。

这项变更将把 initialism 命名规则固化到现有命名治理规格中，并为后续实现提供明确的审查边界、保护对象和验证步骤。

## What Changes

- 扩展 `project-naming-consistency`，要求手写 Go 标识符、godoc 注释、测试名称、文档和 OpenSpec 引用遵循 Go initialism 规范。
- 审查 `common/`、`user-services/`、`docs/`、`openspec/specs/` 和活跃 `openspec/changes/` 中的 initialism 命名与引用一致性。
- 将低风险内部 Go 符号统一为规范拼写，例如 `UserID`、`API`、`HTTP`、`URL`、`JSON`、`UUID`、`JWT`、`TTL`、`SQL`。
- 同步修正受影响的 godoc 注释、测试名称、文档说明、OpenSpec 规格和 capability map 引用。
- 保持外部契约不变，包括 HTTP 路径、JSON tag、请求参数、header、配置 key、环境变量、Redis key、数据库字段、migration 历史和 Swagger path。
- 不手写修改 `user-services/ent/` 下的 Ent 生成代码，不重命名 Atlas migration 文件或修改迁移校验历史。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `project-naming-consistency`: 增加 Go initialism 命名治理要求，明确内部 Go 符号、注释和文档引用的规范拼写，以及外部契约、生成代码和迁移历史的保护边界。

## Impact

- 影响范围：手写 Go 代码、测试、godoc 注释、文档、OpenSpec 主规格和活跃 change artifacts 中的命名与引用。
- API 兼容性：不改变 HTTP API、JSON 字段、错误码、响应信封、认证语义、配置 key、环境变量或数据库 schema。
- 数据与迁移：不修改 Atlas migration 历史、`atlas.sum` 或已有数据库字段语义。
- 依赖与工具链：不新增运行时依赖；实现阶段应根据实际修改运行 `gofmt`、受影响 Go 模块测试，并在适用时运行 lint 检查。
