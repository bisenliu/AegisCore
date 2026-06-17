# Tasks

## 1. Preparation

- [x] 1.1 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、本 change 的 `proposal.md` 与 `design.md`。
- [x] 1.2 确认本 change 使用 `docs/changes/expose-user-service-metrics-endpoint/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 1.3 梳理 `common/runtime/observability/metrics` 现有 `Provider`、`NewFxProvider`、`Gatherer()` 和 disabled 行为。
- [x] 1.4 梳理 `user-service/internal/providers/fx.go`、`routes.go`、`gin.go` 和 `user-service/internal/router/router.go` 当前路由组装顺序。
- [x] 1.5 梳理现有 route/provider/scanner 测试，确认新增测试落点。

## 2. Metrics Provider Wiring

- [x] 2.1 在 `user-service/internal/providers/fx.go` 接入 `common/runtime/observability/metrics.NewFxProvider`。
- [x] 2.2 在 `RegisterRouteParams` 中接收 metrics provider。
- [x] 2.3 将 metrics provider 与 `config.Config.Observability.Metrics` 传入 `router.RouteParams`。
- [x] 2.4 如将 `RegisterRoutes` 改为返回 error，同步调整 Fx invoke 和所有调用点测试。
- [x] 2.5 确认 provider wiring 不注册业务指标、不创建后台 goroutine、不改变 datastore/Ent/RBAC 生命周期。

## 3. Router Metrics Route

- [x] 3.1 在 `user-service/internal/router` 新增 metrics route helper，保持 router 层不导入 feature、Ent、Redis 或 SQL。
- [x] 3.2 在 `RouteParams` 增加 metrics config/provider 输入。
- [x] 3.3 在 `RegisterUserServiceHTTPRoutes` 中于 root-level 挂载 metrics route，且不放入 `/api/v1`。
- [x] 3.4 metrics disabled 或 provider disabled 时不注册 metrics route。
- [x] 3.5 metrics enabled 时使用 `promhttp.HandlerFor(provider.Gatherer(), ...)` 输出 Prometheus text format。
- [x] 3.6 不使用 Prometheus 默认全局 registry 或默认 gatherer。
- [x] 3.7 增加 metrics path 防御性校验，覆盖空路径、非 `/` 开头、Gin 参数/通配符、健康探针冲突、OpenAPI/docs/pprof 冲突和 `/api/v1` 冲突。
- [x] 3.8 配置冲突时 fail fast，避免静默覆盖已有 route。

## 4. Access Log And Tracing Noise Control

- [x] 4.1 将 `skipSuccessfulHealthProbeLog` 扩展为 runtime endpoint skip 逻辑，覆盖成功 metrics scrape。
- [x] 4.2 保留失败请求 access log，确保 scrape 失败仍可见。
- [x] 4.3 评估并实现 metrics path tracing skip，避免 Prometheus scrape 产生大量 server span。
- [x] 4.4 保持 `/livez`、`/readyz`、`/startupz` 现有 access log/tracing skip 行为不变。
- [x] 4.5 避免将 metrics path 命名或文档表达为 health probe。

## 5. RBAC And Route Scanner

- [x] 5.1 确认 metrics route 不经过 `AuthWithTokenVersionValidator`。
- [x] 5.2 确认 metrics route 不经过 `permissionhttp.Authorize`。
- [x] 5.3 增加 route registration 测试，断言无 token 访问 metrics route 不返回 401。
- [x] 5.4 增加 route registration 测试，断言 metrics route 不触发 RBAC authorizer。
- [x] 5.5 增加 permission route scanner 测试，断言 `/metrics` 和自定义 metrics path 不出现在 authorizable route 集合中。
- [x] 5.6 不修改 `internal/shared/rbacbaseline`，不为 metrics endpoint 增加系统权限。

## 6. Tests

- [x] 6.1 新增或更新 router/provider 测试：metrics disabled 时不注册 configured route。
- [x] 6.2 新增或更新 router/provider 测试：metrics enabled 默认 `/metrics` 返回 HTTP 200。
- [x] 6.3 断言 metrics response `Content-Type` 为 Prometheus text exposition format。
- [x] 6.4 断言 metrics response body 包含至少一个可 scrape metric family。
- [x] 6.5 新增自定义 metrics path 测试，确认 custom path 生效且默认 `/metrics` 未注册。
- [x] 6.6 新增 metrics path 冲突测试，确认 route registration fail fast。
- [x] 6.7 更新 `user-service/internal/providers/gin_test.go`，覆盖成功 metrics scrape 跳过 access log。
- [x] 6.8 覆盖 metrics scrape 失败或未命中请求仍记录 access log。
- [x] 6.9 回归测试 `/livez`、`/readyz`、`/startupz` 行为不变。

## 7. Documentation

- [x] 7.1 更新 `docs/ARCHITECTURE.md`，说明用户服务启用 metrics 后由服务级 router 暴露 configured path。
- [x] 7.2 更新 `docs/DEVELOPMENT.md`，把“当前不注册 `/metrics` 路由”改为“启用后注册 configured metrics route”。
- [x] 7.3 更新 `user-service/configs/config.yaml` observability metrics 注释。
- [x] 7.4 明确 metrics endpoint 不经过 RBAC，部署层负责网络侧保护。
- [x] 7.5 明确本变更不新增业务指标、ServiceMonitor、PodMonitor、Helm chart 或 Kubernetes deployment artifact。
- [x] 7.6 不更新 OpenAPI 文档，除非实现过程中决定将 runtime endpoint 纳入文档；默认不纳入。

## 8. Verification

- [x] 8.1 格式化修改过的 Go 文件：

```bash
gofmt -w <changed-go-files>
```

- [x] 8.2 运行用户服务相关测试：

```bash
make test-user-service
```

- [x] 8.3 运行架构边界检查：

```bash
make architecture-lint
```

- [x] 8.4 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 8.5 检查最终 diff，确认未修改 Ent generated code、Atlas migration、RBAC baseline、业务 API response 或部署 chart。

## 9. Guardrails

- [x] 9.1 不新增 auth/user/role/permission 业务指标。
- [x] 9.2 不新增 HTTP metrics middleware、scheduler adapter 或 workerpool adapter。
- [x] 9.3 不改变 `/livez`、`/readyz`、`/startupz`。
- [x] 9.4 不把 metrics endpoint 加入 RBAC permission baseline 或 route diff authorizable set。
- [x] 9.5 不暴露 DSN、token、Authorization header、Cookie、SQL、Redis key 或原始错误消息。
- [x] 9.6 不新增 `openspec/` 或 `docs/opsx/`。
