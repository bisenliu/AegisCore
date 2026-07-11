## Why

user-service 当前运行时镜像基于 Alpine，实际包含 shell、`apk`、`wget` 和 `grep`；Docker 构建又把全部源码与依赖清单放在同一缓存失效边界内，CI 中相同提交的镜像还会在独立 runner 重复冷构建。与此同时，真实 PostgreSQL/Redis 测试受 `AEGISCORE_TEST_CONTAINERS` 控制，但现有 CI 未启用该开关，已经掩盖了 HTTP E2E 配置缺失导致的真实失败。

需要把运行时镜像、构建链和集成测试门禁作为同一组 `delivery-operations` 安全基线统一收敛，避免只替换基础镜像后破坏 Compose 健康检查，也避免把缓存优化错误表述为依赖校验或可复现性已经完成。

## What Changes

- 将 user-service 运行时镜像迁移到固定 digest 的 `gcr.io/distroless/static-debian12:nonroot`，显式静态编译服务二进制，并验证运行时仍具备 CA certificates、IANA timezone 和 `/tmp` 所需能力。
- **BREAKING**：统一镜像、Kubernetes 和 Helm 的运行 UID/GID 为 `65532`，移除 Alpine 中创建的命名用户 `aegiscore`、UID/GID `10001` 以及依赖 shell/package manager 的运行时路径。
- 新增 user-service 原生 `healthcheck` CLI，Compose 改为 exec-form 健康检查，不再依赖 `CMD-SHELL`、`wget` 或 `grep`；Kubernetes 和 Helm 继续使用 kubelet HTTP probe。
- 重构 Dockerfile 为 manifest-first 的 BuildKit 构建，固定 builder/runtime 基础镜像 digest，同时缓存 `/go/pkg/mod` 与 Go build cache，并使用只读依赖和可重复构建参数。
- 合并 CI 中同一提交的重复镜像构建，让构建、镜像断言、Trivy 扫描和 SBOM 基于同一镜像工件，并使用持久化 BuildKit cache。
- 修复 user-service E2E harness 的可观测性配置，随后将现有 CI `test` job 改为设置 `AEGISCORE_TEST_CONTAINERS=1` 的阻塞式真实依赖测试门禁。
- 不增加 `TEST_CONTAINERS` 兼容别名，不保留 Alpine/debug 运行时镜像 fallback，也不保留同一提交在独立 CI job 中重复冷构建的兼容路径。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 增加 user-service 最小运行时镜像、原生容器健康检查、缓存友好且可验证的 Docker 构建，以及 Docker-backed 集成测试 CI 门禁要求。

## Impact

- Go 代码：`user-service/cmd/` 新增健康检查命令及测试；`user-service/tests/e2e/harness_test.go` 补齐有效测试配置。
- Docker/CI：`deployments/docker/user-service.Dockerfile`、`.github/workflows/ci.yml`、`.dockerignore` 及必要的镜像验证脚本。
- 部署资产：`deployments/compose/docker-compose.yml`、`deployments/k8s/user-services/`、`deployments/helm/aegiscore-user-services/` 的 UID、健康检查和镜像约束。
- 依赖元数据：`go.work.sum` 与各 workspace module 的 `go.mod`/`go.sum` 需要保持规范化、只读且可用于独立依赖层。
- 文档和规格：`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md`、部署 README 和 `delivery-operations` 主规格归档结果。
- 安全：减少运行时可执行工具和基础镜像供应链漂移，保留现有非 root、只读根文件系统、禁止提权、capabilities drop 和 seccomp 约束。
- 不影响 HTTP API、OpenAPI、Ent schema、Atlas SQL migration、认证/RBAC 业务语义或数据库数据。
