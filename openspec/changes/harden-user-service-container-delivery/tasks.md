## 1. 依赖清单和 Docker 构建输入

- [x] 1.1 规范化 `go.work.sum` 与 workspace module checksum，审查新增条目，并确认后续只读构建不再修改 `go.work.sum`、`common/go.sum` 或 `user-service/go.sum`。
- [x] 1.2 为 builder 与 runtime 基础镜像选择并记录审核后的多架构 digest，保留可读版本 tag，并确认 Renovate 能继续管理 tag/digest 更新。
- [x] 1.3 将 `deployments/docker/user-service.Dockerfile` 重构为 manifest-first BuildKit 构建，先复制 workspace manifests 并准备依赖，再复制 `common`、`tools` 和 `user-service` 源码。
- [x] 1.4 为依赖准备和编译挂载 `/go/pkg/mod` cache，为编译挂载 Go build cache，并使用 `CGO_ENABLED=0`、`-mod=readonly`、`-trimpath` 和显式 VCS metadata 参数构建服务二进制。
- [x] 1.5 将 runtime stage 切换为固定 digest 的 `gcr.io/distroless/static-debian12:nonroot`，删除 `apk`、用户创建和所有 Alpine/runtime shell 逻辑，只复制服务二进制与配置。

## 2. 原生健康检查 CLI

- [x] 2.1 在 `user-service/cmd/` 增加 `healthcheck` 子命令和最小参数模型，支持目标 URL、有限超时和稳定退出码，且不引入 feature、Ent、Redis、Fx 或部署包依赖。
- [x] 2.2 为 `healthcheck` 增加 HTTP 2xx ready、非 ready、非 2xx、非法响应、连接失败、超时和参数错误测试。
- [x] 2.3 更新 root command surface 测试，确认 `healthcheck` 与现有 `serve`、`rbac`、`fxgraph` 命令共同注册，且现有 CLI flag 和退出语义不变。

## 3. Distroless 部署资产迁移

- [x] 3.1 将 `deployments/compose/docker-compose.yml` 的 user-service healthcheck 改为 exec-form 调用原生 `healthcheck` 子命令，删除 `CMD-SHELL`、`wget` 和 `grep` 依赖。
- [x] 3.2 将 Kubernetes Deployment、RBAC seed Job 和相关注释中的 `runAsUser`、`runAsGroup`、`fsGroup` 从 10001 统一改为 65532，并保持只读根文件系统、禁止提权、capabilities drop 和 seccomp 配置。
- [x] 3.3 将 Helm `values.yaml`、环境覆盖和模板渲染结果统一到 UID/GID 65532，确认 Deployment 与 RBAC seed Job 使用同一数值身份。
- [x] 3.4 增加或扩展镜像验证脚本/CI 断言，检查静态链接、UID/GID 65532、CA、`Asia/Shanghai` timezone、`/tmp` 写入、CLI help，以及 shell、apk、wget、curl、grep、Atlas 均不存在。
- [x] 3.5 运行 `docker compose -f deployments/compose/docker-compose.yml config`、Kubernetes YAML/schema 检查、`helm lint deployments/helm/aegiscore-user-services` 和 `helm template`，审查 UID、探针、healthcheck 与命令渲染结果。

## 4. Docker-backed 测试门禁

- [x] 4.1 在 `user-service/tests/e2e/harness_test.go` 生成的测试配置中补齐完整 observability 段，满足 metrics path、tracing exporter 和当前 config validation，不降低生产配置校验。
- [x] 4.2 使用 `AEGISCORE_TEST_CONTAINERS=1` 运行 common 容器 smoke、role PostgreSQL 集成测试和 user-service HTTP E2E，确认当前被跳过的真实依赖测试全部执行且通过。
- [x] 4.3 将 `.github/workflows/ci.yml` 的现有 `test` job 改为设置 `AEGISCORE_TEST_CONTAINERS=1` 的阻塞门禁，调整 timeout，并保持 `make test` 作为仓库聚合入口。
- [x] 4.4 搜索仓库与 CI，确认不存在 `TEST_CONTAINERS` 读取、别名或文档示例，并确认完整门禁不依赖单独的 `AEGISCORE_TEST_E2E`。
- [x] 4.5 记录容器测试的执行测试名与耗时，确认 Docker 前置条件失败会使 CI 失败而不是静默 skip。

## 5. CI 镜像工件和安全验证

- [x] 5.1 合并 `.github/workflows/ci.yml` 中重复的 user-service image 构建路径，让 BuildKit build、镜像内容断言、CLI smoke、Trivy image scan 和 image SBOM 使用同一 image ID/digest。
- [x] 5.2 为 CI BuildKit 配置 GitHub Actions cache backend 或等价 external cache，确认后续提交能复用 module 与编译 cache，且 cache miss 不改变依赖校验结果。
- [x] 5.3 保留 HIGH/CRITICAL 镜像漏洞阻塞门禁和 SBOM artifact，增加 Distroless runtime 内容断言失败时的明确诊断。
- [x] 5.4 以相同提交连续执行构建，确认未变更输入命中 cache；在隔离 build context 修改源码后确认只重新执行必要源码/编译层。

## 6. 文档、规格和完整验证

- [x] 6.1 更新 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md`、Compose/Kubernetes/Helm README，说明 Distroless、UID/GID 65532、原生 healthcheck、BuildKit 和 `AEGISCORE_TEST_CONTAINERS` CI 门禁。
- [x] 6.2 检查 `docs/opsx/CAPABILITY_MAP.md` 中 `delivery-operations` 的路径和说明仍覆盖 Docker、CI、测试与部署资产；如有 drift 则同步更新。
- [x] 6.3 运行 `openspec validate harden-user-service-container-delivery --strict --no-interactive`、`openspec list --specs`、`openspec validate --specs` 和 `make user-service-architecture-lint`。
- [x] 6.4 运行目标测试，包括 `go test ./cmd`、镜像验证、Compose/Kubernetes/Helm 检查和 `AEGISCORE_TEST_CONTAINERS=1 make test`，并确认全部通过。
- [x] 6.5 暂存本次预期代码、部署、文档和 OpenSpec 变更后运行 `make lint`，确认通过后将对应任务标记完成。
- [x] 6.6 保持本次预期变更已暂存，运行 `make verify` 并确认生成物无 drift、工作区无意外未暂存变更后，将 change 视为可归档。
