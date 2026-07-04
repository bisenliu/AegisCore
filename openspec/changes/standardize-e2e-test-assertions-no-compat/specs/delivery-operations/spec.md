## ADDED Requirements

### Requirement: user-service E2E 测试断言迁移
系统 MUST 将 `user-service/tests` 下的 E2E HTTP flow、migration validation 和测试 harness Go 测试迁移到 `docs/TESTING.md` 固化的统一断言规范。断言迁移 MUST 保持 E2E 流程、测试数据构造、Testcontainers 前置条件、migration 应用顺序和生产行为不变，并 MUST NOT 引入旧断言兼容 helper、旧 API 响应兼容断言或机械 `Fail*` 替换。

#### Scenario: 语义化断言优先
- **WHEN** E2E 测试断言 HTTP status、错误、响应 envelope、集合长度、无序集合、JSON 响应、时间相关结果、文件读取、SQL 执行或对象字段
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.Equal`、`require.NotEmpty`、`require.Empty`、`require.Len`、`require.Greater`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration` 或等价语义化断言
- **AND** 存在更具体语义化断言时，测试 MUST NOT 使用 `True`、`False`、手写 if 或多个基础断言拼凑同一检查

#### Scenario: 完整 HTTP flow 独立字段收集
- **WHEN** 完整 HTTP flow 的单个响应包含多个互相独立的字段检查，且后续检查不依赖这些字段全部成功
- **THEN** 测试 MAY 使用 `testify/assert` 收集独立失败
- **AND** 初始化失败、容器或配置前置条件、JSON 解码、数据库连接、migration 应用和后续流程依赖的结果 MUST 使用 `require` 立即终止当前测试

#### Scenario: migration validation 断言迁移
- **WHEN** migration harness 枚举 SQL migration、读取文件、拆分 SQL statement、定位 user-service 根目录或逐条执行 migration
- **THEN** 测试 MUST 使用语义化断言表达错误、空集合、执行失败和路径定位失败
- **AND** 迁移 MUST NOT 改变 SQL parser 对注释、单引号、双引号、dollar quote、statement 分隔和错误返回的处理语义

#### Scenario: 残留失败调用受扫描约束
- **WHEN** 实施完成后扫描 `user-service/tests/**/*_test.go` 中的 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的特殊测试控制流、特殊诊断输出、测试辅助工具边界，或验收正则对 `fmt.Errorf` 的 false positive
- **AND** change tasks MUST 列明每个剩余命中及原因

#### Scenario: E2E 断言迁移验证
- **WHEN** 本 change 实施完成
- **THEN** `rg "github.com/stretchr/testify/(require|assert)" user-service/tests --glob "*_test.go"` MUST 定位到迁移后的实际使用点
- **AND** `go test ./user-service/tests/...` MUST 在具备 E2E 容器前置条件时通过
- **AND** 若容器前置条件不可用，tasks MUST 明确记录 `AEGISCORE_TEST_E2E=1` 或通用容器测试开关、Docker 或兼容容器运行时等可运行前置条件和已完成替代验证
- **AND** `openspec validate standardize-e2e-test-assertions-no-compat` MUST 通过
