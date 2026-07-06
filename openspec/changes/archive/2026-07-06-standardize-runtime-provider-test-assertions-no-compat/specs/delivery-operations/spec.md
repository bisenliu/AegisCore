## ADDED Requirements

### Requirement: user-service 运行时装配测试断言迁移

`user-service/internal/bootstrap` 与 `user-service/internal/providers` 中覆盖 Fx provider、bootstrap validation、PostgreSQL/Redis/Ent provider、Gin engine、routes provider 和 HTTP server lifecycle 的测试 MUST 使用 `docs/TESTING.md` 规定的语义化断言。断言迁移 MUST 保持 Fx 依赖图、provider 输出、生命周期 hook、server start/stop、graceful shutdown、forced close、drain tracker 和配置默认值语义不变。

#### Scenario: Fx provider 和 bootstrap validation 断言

- **WHEN** provider 或 bootstrap 测试验证 `fx.ValidateApp`、named resource、provider 输出、配置默认值、lifecycle hook 数量、启动日志或关闭顺序
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.NotNil`、`require.Len`、`require.Equal`、`require.ElementsMatch` 或等价语义化断言
- **AND** 多个互相独立的 provider 输出或日志字段 MAY 使用 `assert`
- **AND** 迁移 MUST NOT 改变 Fx module、provider、invoke、named resource 或 bootstrap validation 生产行为

#### Scenario: HTTP server lifecycle 断言

- **WHEN** bootstrap 测试验证 listener bind 失败、Serve 错误、Shutdown 错误、lifecycle context cancellation、active handler drain、forced close、default shutdown timeout 或 drain tracker wait 行为
- **THEN** 测试 MUST 使用语义化断言表达错误、错误包含关系、调用次数、耗时边界、日志字段和 server timeout 配置
- **AND** channel handoff、blocked handler、goroutine 退出等待或跨 goroutine 错误传递等测试控制流 MAY 保留符合 `docs/TESTING.md` 例外规则的直接 `testing.T` 失败调用
- **AND** 迁移 MUST NOT 改变 HTTP server start/stop、graceful shutdown、forced close、drain tracker 或 Fx lifecycle 语义

#### Scenario: 残留失败调用受扫描约束

- **WHEN** 目标范围 `_test.go` 保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于并发协调、特殊诊断输出、测试辅助工具边界或无法通过现有语义化断言清晰表达的控制流
- **AND** change tasks MUST 列明剩余例外及原因

### Requirement: 运行时装配断言迁移不扩大交付范围

断言迁移 MUST 只覆盖 issue 指定的 router、providers 和 bootstrap 测试路径。系统 MUST NOT 将本 change 扩展为 feature、cmd、Ent schema、e2e、common、部署资产或 OpenAPI 生成物迁移。

#### Scenario: 实施范围受限

- **WHEN** 实施本 change
- **THEN** 代码修改 MUST 限定在 `user-service/internal/router/**/*_test.go`、`user-service/internal/providers/**/*_test.go`、`user-service/internal/bootstrap/**/*_test.go` 和本 change 的 OpenSpec artifacts
- **AND** change MUST NOT 修改生产 Go 文件、Ent schema、Atlas migration、OpenAPI 生成物、部署清单或 `common` 测试
