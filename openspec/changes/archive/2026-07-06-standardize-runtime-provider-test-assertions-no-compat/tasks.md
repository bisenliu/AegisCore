## 1. 基线与范围确认

- [x] 1.1 确认 `user-service/go.mod` 已直接声明 `github.com/stretchr/testify`，本 change 不新增无关依赖或 tidy 漂移。
- [x] 1.2 记录目标范围内 `t.Fatal` / `t.Error` / `Fail*` 残留扫描和 `testify/require|assert` 使用情况，作为迁移基线。

## 2. router 断言迁移

- [x] 2.1 迁移 `user-service/internal/router/health_test.go` 的 health response、并发检查、timeout 和 checks 顺序断言。
- [x] 2.2 迁移 `user-service/internal/router/metrics_test.go` 的 metrics route、Content-Type、collector context、错误路径和 runtime path 判定断言。
- [x] 2.3 迁移 `user-service/internal/router/openapi_test.go` 与 `pprof_test.go` 的路由、redirect、OpenAPI document、security scheme 和 pprof 状态断言。

## 3. providers 断言迁移

- [x] 3.1 迁移 `user-service/internal/providers/auth_test.go`、`fx_test.go`、`ent_test.go`、`health_test.go` 和 `postgres_test.go` 中 provider、resource、Ent、health check、PostgreSQL 和 Redis 断言。
- [x] 3.2 迁移 `user-service/internal/providers/gin_test.go` 中 Gin engine、request ID、tracing、metrics、panic recovery、response envelope、span 和 helper 断言。
- [x] 3.3 迁移 `user-service/internal/providers/routes_test.go` 中 route registration、public/runtime route、metrics、health、auth/RBAC route、日志和 response envelope 断言。

## 4. bootstrap 断言迁移

- [x] 4.1 迁移 `user-service/internal/bootstrap/http_test.go` 中默认配置、HTTP server lifecycle、serve/shutdown、active request drain、forced close 和 drain tracker 断言。
- [x] 4.2 迁移 `user-service/internal/bootstrap/validation_test.go` 中 Fx validation、composition root 文本、shutdown ordering 和 mock resource lifecycle 断言。

## 5. 例外收敛与记录

- [x] 5.1 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/internal/router user-service/internal/providers user-service/internal/bootstrap --glob '*_test.go'`，消除可用语义化断言表达的剩余命中。
- [x] 5.2 在本文件记录所有保留的 `t.Fatal` / `t.Error` / `Fail*` 例外及原因，确保每个例外符合 `docs/TESTING.md` 的特殊测试控制流、特殊诊断输出或测试辅助工具规则。
- [x] 5.3 运行 `rg "github.com/stretchr/testify/(require|assert)" user-service/internal/router user-service/internal/providers user-service/internal/bootstrap --glob '*_test.go'`，确认目标范围内存在迁移后的实际使用点。

## 6. 验证

- [x] 6.1 运行 `gofmt` 覆盖本次修改的 Go 测试文件。
- [x] 6.2 运行 `go test ./user-service/internal/router ./user-service/internal/providers ./user-service/internal/bootstrap`，确认目标包测试通过。
- [x] 6.3 运行 `openspec validate standardize-runtime-provider-test-assertions-no-compat`，确认 change artifacts 通过 OpenSpec 校验。
- [x] 6.4 如时间允许，运行 `make user-service-architecture-lint`，确认 OPSX 文档语言和架构边界检查通过；若无法运行，记录具体原因。

## 7. 剩余例外记录

- [x] 7.1 剩余例外：无；最终残留扫描未发现 `t.Fatal` / `t.Error` / `Fail*` 命中。
