## ADDED Requirements

### Requirement: user-service CLI 与 Ent schema 测试断言迁移

`user-service/cmd` 与 `user-service/ent/schema` 中覆盖 CLI command、flag/env normalization、cleanup error、Ent schema field、edge、index、annotation、default、validator 和 mixin 的测试 MUST 使用 `docs/TESTING.md` 规定的语义化断言。断言迁移 MUST 保持服务前缀 Make target、CLI command graph、命令帮助输出约束、Ent schema 定义、Atlas migration 和生成物交付流程不变。

#### Scenario: CLI command 断言

- **WHEN** cmd 测试验证 root command、serve command、RBAC command、flag 绑定、env normalization、command output、usage 文本、cleanup error 或执行错误
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.NotNil`、`require.Len`、`require.Equal`、`require.Contains`、`require.Regexp` 或等价语义化断言
- **AND** 多个互相独立的 command property MAY 使用 `assert`
- **AND** 迁移 MUST NOT 新增旧 root command alias、旧 flag/env 名、旧 usage 文本或无服务前缀 Make target 兼容断言

#### Scenario: Ent schema 断言

- **WHEN** Ent schema 测试验证 field 数量、field 名称、类型、唯一性、可选性、默认值、validator、edge、index、annotation、mixin 或 schema comment
- **THEN** 测试 MUST 使用 `require.Len`、`require.Equal`、`require.NotNil`、`require.Empty`、`require.NotEmpty`、`require.ElementsMatch`、`require.Contains`、`require.Greater`、`require.Regexp` 或等价语义化断言
- **AND** 多个互相独立的 field、edge、index 或 annotation 检查 MAY 使用 `assert`
- **AND** 迁移 MUST NOT 修改 Ent schema、Ent 生成代码、Atlas migration 或 schema 运行时行为

#### Scenario: 残留失败调用受扫描约束

- **WHEN** 目标范围 `_test.go` 保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于特殊测试控制流、特殊诊断输出、测试辅助工具边界或无法通过现有语义化断言清晰表达的控制流
- **AND** change tasks MUST 列明剩余例外及原因

### Requirement: cmd 与 Ent schema 断言迁移不扩大交付范围

断言迁移 MUST 只覆盖 issue 指定的 cmd 与 Ent schema 测试路径。系统 MUST NOT 将本 change 扩展为 router/provider/bootstrap、feature、e2e、common、部署资产、OpenAPI 生成物或数据库结构变更。

#### Scenario: 实施范围受限

- **WHEN** 实施本 change
- **THEN** 代码修改 MUST 限定在 `user-service/cmd/**/*_test.go`、`user-service/ent/schema/**/*_test.go` 和本 change 的 OpenSpec artifacts
- **AND** change MUST NOT 修改生产 Go 文件、Ent schema、Ent 生成代码、Atlas migration、OpenAPI 生成物、部署清单或 `common` 测试
