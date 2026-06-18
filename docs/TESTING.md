# AegisCore 测试说明

## 1. 验证入口

| 命令 | 用途 |
|---|---|
| `make test` | 运行 `common` 和 `user-service` 的 Go 测试 |
| `make lint` | 运行各模块 `golangci-lint` |
| `make user-service-architecture-lint` | 检查 user-service 架构边界、生成物 drift 和 OPSX 文档语言约束 |
| `make user-service-openapi-generate` | 生成 OpenAPI 3 文档 |
| `make compose-dashboard-check` | 检查 Compose Grafana dashboard 是否与通用 dashboard 一致 |
| `make verify` | 运行 lint、架构 lint、测试、OpenAPI 生成和 `git diff --exit-code` |

## 2. 单元测试

Go 单元测试位于对应包内，以 `_test.go` 结尾。常见覆盖范围：

- `common/contract/`：错误、响应和分页契约。
- `common/http/`：binding、middleware、OpenAPI 和 response helper。
- `common/runtime/`：config、datastore、logger、metrics、scheduler、workerpool、timezone。
- `common/security/`：JWT、token、token version、Casbin authorizer、password。
- `user-service/internal/features/*/domain`：领域对象和错误。
- `user-service/internal/features/*/application`：command、query、validator、service 和 seed。
- `user-service/internal/features/*/transport/http`：controller、input、mapper 和 routes。

运行单模块测试：

```bash
make common-test
make user-service-test
```

## 3. 集成测试和 e2e

集成和 e2e 测试使用真实依赖时优先复用 `common/testing/containers/`：

- `common/testing/containers/postgres.go`
- `common/testing/containers/redis.go`

user-service e2e 位于 `user-service/tests/e2e/`，覆盖 HTTP flow、migration 和测试 harness。

运行：

```bash
make user-service-test
```

## 4. 架构边界测试

架构检查脚本位于 `user-service/scripts/architecture-lint.sh`，覆盖：

- 禁止旧 RBAC baseline import。
- 禁止 auth、role 直接导入 user domain。
- 禁止 shared package 导入 feature package。
- 禁止 application/domain/infrastructure 导入 feature HTTP transport。
- 检查 OpenAPI 和 Ent 生成物 drift。
- 检查 `openspec/specs/`、`openspec/changes/` 和 `docs/opsx/` 下 Markdown 是否保留默认英文模板内容。

运行：

```bash
make user-service-architecture-lint
```

## 5. OpenAPI drift

API 注解、路由、request、response 或共享 OpenAPI helper 变化后，执行：

```bash
make user-service-openapi-generate
git diff -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml
```

若生成物有变化，应随代码一起提交。

## 6. Ent 和 migration 验证

Ent schema 变化后执行：

```bash
make user-service-generate
make user-service-migrate-diff name=<migration-name>
make user-service-migrate-validate
```

应用到环境前使用：

```bash
DATABASE_URL='<postgres-url>' make user-service-migrate-apply
```

## 7. 观测资产验证

通用 Grafana dashboard 变化后执行：

```bash
make compose-dashboard-generate
make compose-dashboard-check
```

Prometheus alert 或 dashboard 变更需要同时检查 `deployments/observability/` 和 `deployments/compose/` 中的对应资产。

## 8. OPSX 文档和规格验证

变更 OPSX 文档或 OpenSpec specs 后执行：

```bash
openspec list --specs
openspec validate --specs
make user-service-architecture-lint
```

主规格应包含 `Requirement` 和 `Scenario`，并覆盖主流程、异常流程或边界条件。
