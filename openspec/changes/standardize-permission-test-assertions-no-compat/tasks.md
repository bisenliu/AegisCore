## 1. Baseline 扫描

- [x] 1.1 扫描 `user-service/internal/features/permission/**/*_test.go` 中 `t.Fatalf`、`t.Fatal`、`t.Errorf`、`t.Error` 和 `Fail` 类调用，按 package 记录迁移范围。
- [x] 1.2 扫描 permission 测试中现有 `github.com/stretchr/testify/require` 和 `github.com/stretchr/testify/assert` 使用点，确认需要新增或调整 import 的文件。
- [x] 1.3 确认本次只修改 permission `_test.go` 和本 change artifacts，不修改 permission 生产代码、Casbin model、policy sync、Redis watcher、PostgreSQL schema、HTTP API、OpenAPI 或部署资产。

## 2. 基础与边界测试迁移

- [x] 2.1 将 `domain`、feature-level `metrics`、`transport/http/input` 和 `infrastructure/redis/keys` 等低风险测试迁移为 `require` 语义化断言。
- [x] 2.2 将 `transport/http/scanner`、`transport/http/controller` 和 `transport/http/authorization` 测试迁移为 `require`，对多字段响应中互相独立字段失败收集按需使用 `assert`。
- [x] 2.3 保持 HTTP boundary 测试的路由、白名单、认证上下文和 envelope 校验语义不变，不新增旧授权白名单或旧 scanner 输出兼容断言。

## 3. Application 测试迁移

- [x] 3.1 将 `application/command`、`application/query`、`application/authorization`、`application/policy_sync` 和 `application/metrics` 测试中的手写失败判断迁移为 `require` 或必要的 `assert`。
- [x] 3.2 在 route diff 测试中只对互相独立的 missing、stale 和 mismatch 字段使用 `assert` 收集失败；错误返回、nil 检查和前置对象检查继续使用 `require`。
- [x] 3.3 保持已有 gomock 生成物、matcher、expectation 和失败注入方式不变，不回退为手写 collaborator double。

## 4. Infrastructure 测试迁移

- [x] 4.1 将 `infrastructure/casbin` 测试迁移为 `require` 语义化断言，保持授权 allow/deny、wildcard、fail-closed、context cancellation 和 LastError 语义不变。
- [x] 4.2 将 `infrastructure/postgres` store 测试迁移为 `require` 或必要的 `assert`，不修改 Ent schema、migration 或查询生产逻辑。
- [x] 4.3 将 `infrastructure/redis/watcher_test.go` 迁移为 `require` 语义化断言，保留 watcher `Start()` / `Stop(ctx)` 生命周期契约，不新增旧 watcher 签名兼容断言。

## 5. 格式化与残留例外

- [x] 5.1 对修改过的 Go 测试文件运行 `gofmt`，清理未使用 import 并保持生成 mock 文件使用方式不变。
- [x] 5.2 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/permission --glob '*_test.go'`，确认剩余命中均符合 `docs/TESTING.md` 特殊例外。
- [x] 5.3 在实现记录中列明 5.2 的剩余命中和保留原因；如果没有剩余命中，记录为无残留例外。
- [x] 5.4 运行 `rg "github.com/stretchr/testify/(require|assert)" user-service/internal/features/permission --glob '*_test.go'`，确认迁移后的实际使用点可定位。

## 6. 验证

- [x] 6.1 运行 `go test ./user-service/internal/features/permission/...` 并确认通过。
- [x] 6.2 运行 `openspec validate standardize-permission-test-assertions-no-compat` 并确认通过。
- [x] 6.3 将本次预期代码和文档变更加到暂存区后运行 `make lint`，未通过时修复后重跑。
- [x] 6.4 保持本次预期变更已暂存后运行 `make verify`，未通过时修复后重跑，并确认无非预期生成物 drift。
