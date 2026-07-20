## 1. Go 运行时与测试命名

- [x] 1.1 将 `user-service/cmd` 根命令、help 断言和运维 CLI 示例从 `aegiscore-user-services` 改为 `aegiscore-user-service`，不添加旧命令别名。
- [x] 1.2 将 `user-service/configs/config.yaml`、E2E harness、bootstrap/provider/router 测试 fixture 的默认 `app.name`、JWT issuer、日志 `service` 字段、健康响应和 metrics `service` label 更新为 `aegiscore-user-service`。
- [x] 1.3 将 auth Redis key 测试和 permission/RBAC Redis key 测试中的 key prefix 更新为 `aegiscore-user-service`，确认实现只使用当前 `app.name` 生成 key，不读取或双写旧 prefix。
- [x] 1.4 将 `user-service/internal/bootstrap/app.go` Fx module 名和相关测试中无必要的复数服务名统一为 `aegiscore-user-service`，保持 feature 和 package 边界不变。

## 2. 交付资产命名

- [x] 2.1 更新 `deployments/docker/user-service.Dockerfile` 和 `deployments/docker/verify-user-service-image.sh`，默认镜像名使用 `aegiscore-user-service`，容器内二进制使用 `/app/user-service/bin/user-service`。
- [x] 2.2 更新 `.github/workflows/ci.yml`、根 `Makefile`、`deployments/compose/docker-compose.yml`、Compose README 和辅助脚本中的镜像名、healthcheck、CLI 路径和 Redis key prefix。
- [x] 2.3 更新 `deployments/k8s/user-service/` 下原生清单和 README，使资源名、labels、ServiceAccount、ConfigMap、Secret、Job、PDB、HPA、NetworkPolicy、image 和命令路径使用 `aegiscore-user-service` 或 `user-service`。
- [x] 2.4 更新 `deployments/helm/aegiscore-user-service/` chart 元数据、helpers、templates、values、README 和父级 Helm README；必要时将 chart 目录重命名为 `deployments/helm/aegiscore-user-service/` 并同步所有引用。

## 3. 观测、文档与生成物

- [x] 3.1 更新 `deployments/observability/prometheus/user-service-alerts.yaml`、Grafana dashboard、Compose Grafana dashboard 和 Prometheus scrape config，使 rule group、UID、默认变量、查询和 static label 统一为 `aegiscore-user-service`。
- [x] 3.2 更新 `docs/`、`openspec/specs/`、部署 README、开发/测试说明和 OPSX 相关入口中当前主规格引用的 CLI、镜像、Kubernetes、Helm、metrics 和 Redis key prefix。
- [x] 3.3 运行 `make user-service-openapi-generate`，更新 `user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml` 中的服务名示例，并检查生成物 drift。
- [x] 3.4 运行命名残留检查 `rg --hidden -n 'aegiscore-user-services|user-services' --glob '!openspec/changes/standardize-user-service-name/**' --glob '!logs/**' --glob '!.git/**' --glob '!.kilo/**'`，确认当前代码、部署、文档和主规格中不再保留旧运行时契约。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./cmd ./internal/bootstrap ./internal/providers ./internal/features/auth/... ./internal/features/permission/...`。
- [x] 4.2 运行 `make compose-dashboard-check`，确认 Grafana/Prometheus 资产仍可验证。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认命名清理未破坏架构边界。（首次运行因预期 OpenAPI 生成物未暂存触发 drift 检查，暂存预期变更后重跑。）
- [x] 4.4 运行 `openspec validate standardize-user-service-name`、`openspec list --specs` 和 `openspec validate --specs`。
- [x] 4.5 将本次预期代码、部署、文档、生成物和 OpenSpec 变更加到暂存区。
- [x] 4.6 在暂存预期变更后运行 `make lint`。
- [x] 4.7 在暂存预期变更后运行 `make verify`，确认最终 drift 检查通过。
