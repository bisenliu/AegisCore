## Why

用户领域在 DTO、domain、repository、service 和 Ent 生成代码中普遍使用 `Username` 表达用户名概念，但 `user-services/internal/errmsg` 中的错误消息常量使用了 `MsgInvalidUserName`，与项目命名习惯不一致。现在统一该内部 Go 符号命名，可以降低后续维护和搜索成本，同时避免把 `Username` 拆成 `UserName` 的风格继续扩散。

## What Changes

- 将用户服务内用户名相关错误消息常量命名统一为 `Username` 风格，例如 `MsgInvalidUserName` 改为 `MsgInvalidUsername`。
- 查询整个仓库中类似的 `UserName` 拆词式内部 Go 符号，并在低风险范围内同步改为 `Username`。
- 同步更新所有 workspace 内引用点和相关测试，确保重命名后构建与测试通过。
- 保持公开错误消息文案、HTTP 状态码、响应信封、JSON 字段、配置 key、数据库 schema 和 migration 历史不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `project-naming-consistency`: 补充用户名领域术语在内部 Go 符号中应统一使用 `Username` 而非 `UserName` 的命名约束，并要求重命名同步引用且不改变外部契约。

## Impact

- 影响代码：`user-services/internal/errmsg/messages.go` 及所有引用该错误消息常量的 Go 文件。
- 影响审查范围：整个仓库内 `UserName`、`userName` 等类似拆词式内部 Go 符号。
- API 兼容性：不改变 HTTP 路径、JSON 字段、响应码、响应消息文本、配置项、数据库字段或 Ent schema。
- 依赖与数据：不新增依赖，不涉及数据迁移，不修改 Atlas migration 历史。
