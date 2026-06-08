## ADDED Requirements

### Requirement: Use Go initialism casing for identifiers and references

系统 SHALL 在手写 Go 标识符、godoc 注释、测试名称、文档和 OpenSpec 引用中使用符合 Go Code Review Comments 和 Uber Go Style Guide 的 initialism 命名风格。常见 initialism 包括 `ID`、`API`、`HTTP`、`URL`、`JSON`、`UUID`、`JWT`、`TTL` 和 `SQL`。命名标准化 MUST 保持外部可观察契约不变。

#### Scenario: Internal Go symbol uses canonical initialism spelling
- **WHEN** 手写 Go 类型、函数、方法、变量、常量或测试名称包含常见 initialism
- **THEN** 标识符 MUST 使用 `UserID`、`API`、`HTTP`、`URL`、`JSON`、`UUID`、`JWT`、`TTL`、`SQL` 等 Go 风格拼写
- **THEN** 标识符 MUST NOT 使用 `UserId`、`Api`、`Http`、`Url`、`Json`、`Uuid`、`Jwt`、`Ttl`、`Sql` 等非 Go 风格拼写

#### Scenario: Unexported Go symbol preserves initialism word casing
- **WHEN** 未导出的 Go 标识符包含常见 initialism
- **THEN** 标识符 MUST 使用 `userID`、`apiClient`、`httpServer`、`jsonBody`、`uuidValue`、`jwtToken`、`ttl`、`sqlDB` 等 Go 风格拼写
- **THEN** 标识符 MUST NOT 使用 `userId`、`apiCLIENT`、`httpSERVER`、`jsonBODY`、`uuidVALUE`、`jwtTOKEN`、`tTL`、`sqlDb` 等混合或错误拼写

#### Scenario: Comments and docs match renamed identifiers
- **WHEN** Go 标识符因 initialism 规则被重命名
- **THEN** 对应 godoc 注释 MUST 以重命名后的标识符开头
- **THEN** 测试名称、文档、OpenSpec 规格和 capability map 中的内部符号引用 MUST 同步更新
- **THEN** 注释和文档 MUST NOT 将外部字段名误改为 Go 标识符风格

#### Scenario: External contract names are preserved
- **WHEN** initialism 候选涉及 HTTP 路径、JSON tag、query/path 参数、header、配置 key、环境变量、Redis key、数据库字段、migration 文件名或 Swagger path
- **THEN** 实现 MUST 保留现有外部契约字符串，例如 `user_id`、`session_id`、`token_version` 和 `X-Trace-ID`
- **THEN** 如需改变外部契约名称，MUST 作为单独 breaking change 提出

#### Scenario: Generated and migration artifacts are not hand-edited
- **WHEN** initialism 候选位于 Ent 生成代码、Atlas migration 文件名或迁移校验历史
- **THEN** 实现 MUST 不手写修改生成代码
- **THEN** 实现 MUST 不重命名 migration 文件或修改 migration 历史

#### Scenario: Package names remain lowercase
- **WHEN** Go package name 或 import path 包含缩写词或 initialism 相关语义
- **THEN** package name MUST 继续遵循 Go 包名规则，使用短小、全小写、无下划线的名称
- **THEN** initialism 规范 MUST NOT 导致 package name 使用大写字母或混合大小写
