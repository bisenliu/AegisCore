## Why

`user-service/cmd` 和 `user-service/ent/schema` 的历史 Go 测试仍存在 `t.Fatal`、`t.Errorf`、手写 if 断言和泛化布尔断言，导致 CLI 命令、flag/env 归一化、cleanup error 以及 Ent schema field/index 测试的失败信息和前置条件处理不一致。

当前 `docs/TESTING.md` 与 `delivery-operations` 主规格已经固化统一断言规范，需要把 cmd 与 Ent schema 测试集中迁移到语义化 `testify/require` / `assert`，同时明确不为旧命令名、旧 flag/env 名或旧 schema 形态新增兼容断言。

## What Changes

- 将 `user-service/cmd/**/*_test.go` 中 CLI root/RBAC 命令、flag/env normalization、cleanup error 和 command contract 测试迁移为 `testify/require` 语义化断言。
- 将 `user-service/ent/schema/**/*_test.go` 中 schema field、edge、index、annotation、default、validator 和 mixin 测试迁移为语义化断言。
- 对多个独立 command property 或 schema field/index 检查，按 `docs/TESTING.md` 使用 `testify/assert` 收集独立失败。
- 优先使用 `require.Len`、`require.Greater`、`require.ErrorContains`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp` 等更具体断言，避免用 `True` / `False` 或手写 if 包装可由专属断言表达的检查。
- 保留并记录确实属于特殊测试控制流、特殊诊断输出或测试辅助工具边界的直接 `testing.T` 失败调用例外。
- 不修改 CLI 命令行为、服务前缀 Make target、Ent schema、Atlas migration、OpenAPI 生成物或部署资产。
- 不新增旧 root command alias、旧 flag/env 名、无服务前缀 Make target 兼容断言、机械 `Fail` / `Failf` / `FailNow` / `FailNowf` 替换或旧手写断言兼容 helper。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 明确 user-service CLI 命令测试和 Ent schema 测试必须遵循统一 Go 测试断言规范，并保持服务前缀 Make target、CLI 命令契约和 schema 交付流程不变。
- `rbac-access-control`: 明确 RBAC CLI seed、assign-super-admin、create-super-admin 等命令测试的断言表达约束，不新增旧 RBAC CLI 行为兼容断言。
- `shared-platform-primitives`: 明确 cmd 与 Ent schema 测试直接使用 `testify/require` / `assert`，不得为历史断言迁移新增共享兼容 helper 或生产测试专用 API。

## Impact

- 影响测试代码范围限定为 `user-service/cmd/**/*_test.go` 和 `user-service/ent/schema/**/*_test.go`。
- `user-service/go.mod` 已直接声明 `github.com/stretchr/testify`，预计无需新增依赖；若 tidy 产生与本 change 无关的漂移不得提交。
- 不影响生产 CLI 行为、root command 名称、RBAC seed 语义、超级管理员引导、Ent schema、Ent 生成代码、Atlas migration、OpenAPI、HTTP API、部署资产或运行时配置。
- 验证命令包括断言残留扫描、`rg "github.com/stretchr/testify/(require|assert)" user-service/cmd user-service/ent/schema --glob '*_test.go'`、`go test ./user-service/cmd ./user-service/ent/schema` 和 `openspec validate standardize-cmd-schema-test-assertions-no-compat`。
