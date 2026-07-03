## Context

`docs/TESTING.md` 已规定 Go 测试断言优先使用 `testify/require`，仅在需要收集多个互相独立字段失败时使用 `testify/assert`。当前 `user-service/internal/features/role/**/*_test.go` 和 `user-service/internal/shared/rbacbaseline/**/*_test.go` 仍保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf` 和手写条件失败判断，覆盖 role command/query/seed、domain、HTTP boundary、PostgreSQL store、用户角色绑定、角色权限绑定和 baseline catalog 测试。

本次 change 属于 `rbac-access-control` capability 的测试契约标准化。它只调整测试表达和测试约束，不改变 role 生产代码、RBAC seed 语义、超级管理员绑定、baseline catalog 内容、Casbin policy sync、PostgreSQL schema、HTTP API、OpenAPI 或部署资产。

## Goals / Non-Goals

**Goals:**

- 将 role feature 与 shared RBAC baseline 历史测试中的手写失败判断迁移为 `require` 语义化断言。
- 在多字段 HTTP response 或 baseline catalog 等互相独立字段校验场景中，允许使用 `assert` 收集失败信息。
- 保持现有 gomock 生成物和 collaborator expectation 表达方式，避免回退为手写 store/notifier double。
- 明确保留的 `t.Fatal`、`t.Errorf` 或 `Fail` 类调用必须符合 `docs/TESTING.md` 的特殊例外，并在 tasks 中列明核查要求。

**Non-Goals:**

- 不修改 role 生产行为、RBAC seed、超级管理员绑定、PostgreSQL schema、Casbin policy sync 或 baseline catalog 内容。
- 不迁移 permission、auth/user、router/cmd/e2e 或 `common` 测试。
- 不新增旧 role 字段、旧 binding 行为、旧 baseline 兼容断言或旧 fake 兼容 helper。
- 不新增机械 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf` 兼容 helper。

## Decisions

1. 以 `require` 作为默认迁移目标。

   理由：role 测试多数断言存在前置依赖，例如错误检查后再访问返回对象、HTTP status 失败后再解析响应体、store 初始化失败后再验证关系数据。`require` 的立即失败语义能减少级联失败和空指针风险。

   备选方案：统一使用 `assert`。该方案能一次收集更多失败，但会让依赖前置条件的测试继续执行，和 `docs/TESTING.md` 的默认策略不一致。

2. 对互相独立的字段集合允许使用 `assert`。

   理由：HTTP envelope、角色列表响应、权限摘要或 baseline catalog 中，多个字段失败互不依赖时，一次性呈现差异能提高诊断效率。

   备选方案：全部使用 `require`。该方案更简单，但会隐藏同一响应或 catalog 中后续字段差异，不适合诊断型输出。

3. 不引入测试兼容层或自定义断言 helper。

   理由：本次目标是移除历史手写断言和旧兼容形态，而不是增加新的抽象层。直接使用 testify 语义化断言能保持测试意图可读。

   备选方案：新增 `mustNoError`、`requireRole` 或 baseline catalog helper。该方案会减少局部重复，但容易重新隐藏断言语义，并可能成为旧字段或旧 catalog 兼容入口。

4. 保持 gomock 生成物使用方式不变。

   理由：role application 和 HTTP boundary 测试已经通过生成 mock 表达 port 调用和失败注入。本次只迁移断言表达，不改变 collaborator 契约或调用顺序验证方式。

   备选方案：改写为手写 fake、spy 或 store double。该方案超出范围，并会削弱对现有 port expectation 的直接约束。

## Risks / Trade-offs

- [Risk] 机械替换可能把需要继续执行的字段校验改成过早终止。→ Mitigation: 对 HTTP response、列表摘要和 baseline catalog 测试逐个判断字段依赖关系，必要时使用 `assert`。
- [Risk] 测试中仍可能残留符合例外的 `t.Fatal` 或 `t.Errorf`。→ Mitigation: 用 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/role user-service/internal/shared/rbacbaseline --glob '*_test.go'` 核查，并在 tasks 中要求列明每个保留例外。
- [Risk] 引入 testify import 后出现未使用 import 或 gofmt drift。→ Mitigation: 分包运行 `gofmt`，并执行 role 与 rbacbaseline 目标 Go 测试。
- [Risk] 误改 mock 生成物或生产文件。→ Mitigation: 限定修改 `user-service/internal/features/role/**/*_test.go` 和 `user-service/internal/shared/rbacbaseline/**/*_test.go`，不手写 `mock_*.go` 生成逻辑，不改生产代码。

## Migration Plan

1. 按 package 扫描 role 与 rbacbaseline 测试中的手写失败判断和现有 testify 使用情况。
2. 从低风险 domain、baseline catalog 和 input 测试开始迁移，再迁移 application、seed、transport/http 和 PostgreSQL store 测试。
3. 每个 package 迁移后运行 `gofmt`，保持 import 和格式稳定。
4. 用 acceptance `rg` 命令核查残留手写失败判断，并记录符合 `docs/TESTING.md` 例外的剩余项。
5. 运行 `go test ./user-service/internal/features/role/... ./user-service/internal/shared/rbacbaseline/...` 和 `openspec validate standardize-role-baseline-test-assertions-no-compat`。

回滚方式：本次只改测试和 OpenSpec change artifacts；若迁移后出现不可接受的测试行为变化，可按 package 回退对应 `_test.go` 断言修改，不需要数据库、部署或 API 回滚。

## Open Questions

- 无待决问题；实现阶段若发现某个残留 `t.Fatal` 属于自定义测试控制流或特殊诊断输出，应在实现记录中说明其符合 `docs/TESTING.md` 例外。
