## 1. 依赖与基线扫描

- [x] 1.1 检查 `user-service/go.mod` 是否已显式声明 `github.com/stretchr/testify` 测试依赖；如需调整，运行 `cd user-service && go mod tidy`，确认 `go.mod` / `go.sum` 只出现与本次断言迁移相关的漂移。
- [x] 1.2 记录迁移前目标范围内的 `t.Fatal` / `t.Error` / `Fail` 命中和 `testify/require|assert` 使用情况，作为收敛基线。

## 2. user domain 与 shared identity 断言迁移

- [x] 2.1 迁移 `user-service/internal/features/user/domain` 测试断言，使用 `require` 表达用户状态、ID、用户名、昵称、软删除和错误预期。
- [x] 2.2 迁移 `user-service/internal/shared/identity` 测试断言，使用 `require` 表达账号状态判断、访问/认证允许性和 identity 错误语义。

## 3. user transport 与 infrastructure 断言迁移

- [x] 3.1 迁移 `user-service/internal/features/user/transport/http` 测试断言，覆盖 input preparer、controller status、response envelope、pagination、response data 和错误映射；多个独立 response 字段可按 `docs/TESTING.md` 使用 `assert`。
- [x] 3.2 迁移 `user-service/internal/features/user/infrastructure/postgres` 测试断言，覆盖创建、查询、列表、软删除过滤、状态过滤、cursor 分页、错误映射和实体字段匹配。
- [x] 3.3 检查 `user-service/internal/features/user/application` 目标测试，如存在历史手写断言则迁移为 `require` / `assert`，并保持 command/query/validator 生产行为不变。

## 4. 例外收敛与文档化

- [x] 4.1 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/user user-service/internal/shared/identity --glob '*_test.go'`，消除可用语义化断言表达的剩余命中。
- [x] 4.2 在本文件记录所有保留的 `t.Fatal` / `t.Error` / `Fail` 例外及原因，确保每个例外符合 `docs/TESTING.md` 的特殊测试控制流、特殊诊断输出或测试辅助工具规则。
- [x] 4.3 运行 `rg "github.com/stretchr/testify/(require|assert)" user-service/internal/features/user user-service/internal/shared/identity --glob '*_test.go'`，确认目标范围内存在迁移后的实际使用点。

## 5. 验证

- [x] 5.1 运行 `go test ./user-service/internal/features/user/... ./user-service/internal/shared/identity/...`，确认 user 与 shared identity 目标测试通过。
- [x] 5.2 运行 `openspec validate standardize-user-identity-test-assertions-no-compat`，确认 change artifacts 通过 OpenSpec 校验。
- [x] 5.3 运行 `make user-service-architecture-lint`，确认 OPSX 文档语言、生成物 drift 和架构边界检查通过。
- [x] 5.4 将本次预期代码、测试和 OpenSpec 变更加到暂存区后运行 `make lint`，确认 lint 通过且不被预期 diff 影响。
- [x] 5.5 保持本次预期变更已暂存，运行 `make verify`，确认完整验证通过或记录无法运行的具体原因。

## 6. 剩余例外记录

- [x] 6.1 剩余例外：无；最终扫描未发现 `t.Fatal` / `t.Error` / `Fail` 命中。
