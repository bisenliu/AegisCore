## 1. 依赖与基线扫描

- [x] 1.1 在 `user-service/go.mod` 中显式声明 `github.com/stretchr/testify` 测试依赖，运行 `cd user-service && go mod tidy`，确认 `go.mod` / `go.sum` 只出现与本次断言迁移相关的漂移。
- [x] 1.2 记录迁移前目标范围内的 `t.Fatal` / `t.Error` / `Fail` 命中和 `testify/require|assert` 使用情况，作为收敛基线。

## 2. auth application 与 domain 断言迁移

- [x] 2.1 迁移 `user-service/internal/features/auth/application/authctx` 测试断言，使用 `require` 表达 session/client context 解析和错误预期。
- [x] 2.2 迁移 `auth/application/credentials`、`auth/application/tokens` 和 `auth/application/validators` 测试断言，覆盖 credential 校验、token 签发解析、session policy 和 token version validator。
- [x] 2.3 迁移 `auth/application/command` 和 `auth/application/sessions` 测试断言，覆盖登录、refresh、强制改密、改密、退出当前会话、退出全部会话和 session lifecycle。
- [x] 2.4 迁移 `user-service/internal/features/auth/fx_test.go`、`auth/application/metrics_test.go` 和 `auth/metrics_test.go` 中的配置、metrics collector 和 provider 断言。

## 3. auth transport 与 infrastructure 断言迁移

- [x] 3.1 迁移 `user-service/internal/features/auth/transport/http` 测试断言，覆盖 input preparer、controller status、response envelope、强制改密响应和错误映射，不新增旧兼容字段断言。
- [x] 3.2 迁移 `user-service/internal/features/auth/infrastructure/postgres` 测试断言，覆盖 credential 查询、token version、credential update、软删除和实体字段匹配。
- [x] 3.3 迁移 `user-service/internal/features/auth/infrastructure/redis` 测试断言，覆盖 refresh session、token version cache、TTL、Redis key schema、Lua script metadata 和 legacy key 拒绝。
- [x] 3.4 迁移 `user-service/internal/providers/auth_test.go` 中 JWT、password service 和 auth provider 构造断言。

## 4. 例外收敛与文档化

- [x] 4.1 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/auth user-service/internal/providers/auth_test.go --glob '*_test.go'`，消除可用语义化断言表达的剩余命中。
- [x] 4.2 在本文件记录所有保留的 `t.Fatal` / `t.Error` / `Fail` 例外及原因，确保每个例外符合 `docs/TESTING.md` 的特殊测试控制流、特殊诊断输出或测试辅助工具规则。
- [x] 4.3 运行 `rg "github.com/stretchr/testify/(require|assert)" user-service/internal/features/auth user-service/internal/providers/auth_test.go --glob '*_test.go'`，确认目标范围内存在迁移后的实际使用点。

## 5. 验证

- [x] 5.1 运行 `go test ./user-service/internal/features/auth/... ./user-service/internal/providers`，确认 auth 与 provider 目标测试通过。
- [x] 5.2 运行 `openspec validate standardize-auth-test-assertions-no-compat`，确认 change artifacts 通过 OpenSpec 校验。
- [x] 5.3 运行 `make user-service-architecture-lint`，确认 OPSX 文档语言、生成物 drift 和架构边界检查通过。
- [x] 5.4 将本次预期代码、测试和 OpenSpec 变更加到暂存区后运行 `make lint`，确认 lint 通过且不被预期 diff 影响。
- [x] 5.5 保持本次预期变更已暂存，运行 `make verify`，确认完整验证通过或记录无法运行的具体原因。

## 6. 剩余例外记录

- [x] 6.1 剩余例外：无；最终扫描未发现 `t.Fatal` / `t.Error` / `Fail` 命中。
