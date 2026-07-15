## 1. 前置条件与预算基线

- [x] 1.1 确认 `fix-user-service-internal-shutdown` 的实现与验证已完成并按 OPSX 流程归档；若仍位于活动 change 目录，先单独完成其归档再实施本 change。
- [x] 1.2 对照 `user-service/configs/config.yaml` 和实际 Fx lifecycle 注册点复核 120 秒 Stop 总预算、HTTP 25 秒、auth session purge workerpool 30 秒，以及 watcher、pprof、tracing、Ent/PostgreSQL/Redis/logger 使用 Fx 剩余 context 的关闭边界；若实现已漂移，先同步更新本 change 的 design/specs。

## 2. 配置一致性自动测试

- [x] 2.1 在 `user-service/internal/config/` 增加 termination budget 配置一致性测试，使用结构化 YAML parser 读取应用默认 `runtime.lifecycle.stop_timeout`、原生 Kubernetes `spec.template.spec.terminationGracePeriodSeconds` 和 Helm `deployment.terminationGracePeriodSeconds`，不得使用正则或注释文本提取目标值。
- [x] 2.2 实现并测试 `deployment grace >= Fx Stop budget + 30s` 及原生 Kubernetes/Helm 默认值相等的断言，错误信息包含来源与实际预算，覆盖缺失字段、无效 duration、grace 小于最低值和两个部署入口漂移的表驱动负例。
- [x] 2.3 让真实仓库默认文件进入同一测试路径，先确认当前 35 秒配置会触发预算不足，再随部署默认值更新使测试通过；测试不得要求新增正式构建中的生产 helper 或运行时读取 Pod spec。

## 3. 部署预算与文档同步

- [x] 3.1 将 `deployments/k8s/user-services/deployment.yaml` 的 `terminationGracePeriodSeconds` 从 35 调整为 150，并更新中文注释，明确其覆盖 120 秒 Fx Stop 总预算和 30 秒平台余量，而非只覆盖 HTTP shutdown timeout。
- [x] 3.2 将 `deployments/helm/aegiscore-user-services/values.yaml` 的默认 `deployment.terminationGracePeriodSeconds` 同步调整为 150，并确认 template 继续只从该 value 渲染 Pod grace，不引入独立漂移默认值。
- [x] 3.3 更新 `deployments/k8s/`、`deployments/k8s/user-services/` 与 Helm 相关 README 中的终止说明，记录正常关闭可提前退出、`preStop` 必须计入 30 秒余量、组件局部 timeout 不能替代 Fx 总预算，以及环境 values 覆盖值的校验责任。
- [x] 3.4 核对实现不修改 `runtime.lifecycle.stop_timeout`、任何组件 shutdown timeout、Fx `OnStop` 逆序串行语义、业务 API、OpenAPI、认证/RBAC、Ent schema、Atlas migration、Prometheus 或 Grafana 资产。

## 4. 定向验证与规格检查

- [x] 4.1 对新增 Go 测试执行 `gofmt`，运行 `cd user-service && go test ./internal/config -count=1`，确认真实默认文件和全部预算负例通过。
- [x] 4.2 运行 `helm lint deployments/helm/aegiscore-user-services`、使用默认 values 执行 `helm template`，并执行仓库已有的 Kubernetes kustomize/YAML 解析检查，确认两个渲染结果的 Pod grace 均为 150 秒。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认配置测试、部署资产和文档更新未破坏架构边界。
- [x] 4.4 核对 proposal、design、`delivery-operations` 与 `runtime-observability` delta specs 和实现一致，并运行 `openspec validate align-user-service-termination-budget`。

## 5. 最终交付门禁

- [x] 5.1 在实现、测试、规格和文档任务全部完成后，使用 `git add` 仅暂存前置 change 的归档及主规格合并、`user-service/go.mod`、`user-service/internal/config/`、本 change 修改的 `deployments/` 文件和 `openspec/changes/align-user-service-termination-budget/`，再用 `git diff --cached --check` 与 `git diff --cached --name-only` 检查内容和暂存范围。
- [x] 5.2 在预期变更已暂存后运行 `make lint`；未通过时修复并重新暂存，未通过前不得勾选本任务。
- [x] 5.3 在预期变更已暂存且 lint 通过后运行 `make verify`，通过最终 drift 检查暴露生成物漂移或未暂存意外变更；未通过时修复、重新暂存并重新执行，未通过前不得将 change 视为完成。
