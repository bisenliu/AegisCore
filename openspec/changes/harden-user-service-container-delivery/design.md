## Context

`deployments/docker/user-service.Dockerfile` 当前使用 `golang:1.26.5-alpine3.24` 构建，并以 `alpine:3.24` 作为运行时。运行阶段通过 shell-form `RUN apk add` 安装 `tzdata` 并创建 UID/GID 10001 的 `aegiscore` 用户；实际镜像同时包含 shell、包管理器和 BusyBox 工具。当前服务二进制已经可以静态链接运行，Distroless static 镜像也包含服务所需的 CA certificates、IANA timezone 数据和可写 `/tmp`，因此迁移不存在原理性阻塞。

不能只替换运行阶段 `FROM`：Distroless 没有 shell、`apk` 或命名用户 `aegiscore`，而 Compose 还使用 `CMD-SHELL` 调用 `wget | grep` 检查 `/readyz`。Kubernetes 与 Helm 使用 kubelet `httpGet` probe，不依赖镜像内工具，但它们目前固定 UID/GID 10001，需要与 Distroless `nonroot` 的 65532 统一。

Docker 构建目前在复制 `go.work` 后直接复制 `common`、`tools` 和 `user-service` 全目录，再执行唯一的 `go build`。模块 checksum 并未缺失，但任意源码或非编译文件变化都会使依赖下载和编译层失效。CI 的普通 image job 与 Trivy/SBOM job 位于独立 runner，都会重新构建同一提交的镜像，且没有 external BuildKit cache。

真实容器测试的规范开关是 `AEGISCORE_TEST_CONTAINERS`。CI 未设置该开关时，common PostgreSQL/Redis smoke test、role PostgreSQL 锁语义测试和 user-service HTTP E2E 会通过 `t.Skip` 静默退出；使用正确开关后，当前 E2E 会因 harness 缺少 `observability.metrics.path` 和 `observability.tracing.exporter` 而在 Fx 启动前失败。

## Goals / Non-Goals

**Goals:**

- 让 user-service 生产运行时镜像不包含 shell、包管理器、下载工具、文本处理工具或 Atlas CLI。
- 统一镜像、Compose、Kubernetes 和 Helm 的非 root 数值身份，并保持只读根文件系统与 `/tmp` 写入语义。
- 用应用原生健康检查命令替代 Compose 的 shell pipeline，同时保持现有 `/livez`、`/readyz`、`/startupz` HTTP 契约。
- 将 Go module 下载和源码编译拆成稳定缓存边界，固定基础镜像 digest，并让同一 CI 提交只生成一个被扫描和验证的镜像工件。
- 让真实 PostgreSQL/Redis 测试成为 PR/push 阻塞门禁，并先修复当前被跳过掩盖的 E2E 配置失败。
- 为以上行为提供可自动验证的镜像、Compose、Kubernetes、Helm 和 CI 断言。

**Non-Goals:**

- 不修改 HTTP API、健康端点响应协议、OpenAPI 生成物、Ent schema、Atlas migration 或业务数据。
- 不把健康检查实现放入 `common`，也不引入通用外部探针框架；该命令属于 user-service CLI 交付面。
- 不为 `TEST_CONTAINERS`、Alpine runtime、UID/GID 10001、Distroless debug tag 或 shell healthcheck 保留兼容分支。
- 不要求 race 和 coverage job 重复启动真实容器；真实依赖由单独的阻塞 test job 负责。
- 不把 BuildKit cache 当作依赖锁定或供应链验证的替代品。

## Decisions

### Decision: 使用固定 digest 的 Distroless static nonroot 运行时

运行阶段使用带可读 tag 和不可变 digest 的 `gcr.io/distroless/static-debian12:nonroot`，构建阶段显式设置 `CGO_ENABLED=0`。镜像只复制服务二进制和运行配置，不复制 shell、BusyBox、包管理器、Atlas 或调试工具。

选择 static 变体是因为当前生产依赖不需要 libc 动态链接，同时它保留 CA certificates、timezone data、`/etc/passwd` 中的 nonroot 身份和 `/tmp`。不采用 `scratch`，因为手工维护 CA、timezone、passwd 和临时目录会扩大交付维护面；不采用 debug tag，因为它重新引入 shell。

### Decision: 全部部署资产统一使用 UID/GID 65532

Dockerfile 使用 `USER 65532:65532`，Kubernetes 与 Helm 同步更新 `runAsUser`、`runAsGroup` 和 `fsGroup`。Compose 不再覆盖用户，直接继承镜像身份。实现不保留 10001 的分支或双 UID 文件权限处理。

### Decision: 用 user-service 原生 CLI 执行 Compose 健康检查

在 `user-service/cmd` 增加 `healthcheck` 子命令，接收目标 URL 和超时，使用 Go HTTP client 请求 `/readyz`。HTTP 2xx 且响应表示 ready 时退出 0；网络错误、超时、非 2xx、无效响应或非 ready 状态退出非 0。Compose 使用 exec-form `CMD` 调用该命令，Kubernetes/Helm 继续使用 kubelet `httpGet`。

备选方案是复制额外静态 curl/wget 二进制。该方案增加供应链和漏洞维护面，也让健康检查依赖外部命令语义，因此不采用。

### Decision: Dockerfile 使用 manifest-first BuildKit 构建

构建先复制 `go.work`、`go.work.sum`、三个 workspace module 的 `go.mod` 以及存在的 `go.sum`，执行独立依赖准备；再复制源码并构建。依赖准备和编译都挂载 `/go/pkg/mod`，编译额外挂载 Go build cache。正式构建使用 `-mod=readonly`、`-trimpath` 和显式 VCS metadata 策略，builder/runtime 基础镜像都固定 digest。

在启用只读依赖前，先规范化并提交 workspace checksum，确保依赖准备不会在容器内静默补写清单。该设计只支持 BuildKit，不保留 legacy builder Dockerfile 路径。

### Decision: CI 对同一提交只构建一次运行时镜像

移除独立且重复的 image 冷构建路径，将 BuildKit build、镜像内容断言、CLI smoke、Trivy image scan 和 image SBOM 放入同一阻塞 job，并通过 GitHub Actions cache backend 或等价 external cache 持久化 BuildKit cache。扫描和 SBOM 必须引用刚构建的同一 image ID 或不可变 digest。

### Decision: 现有 test job 成为真实依赖门禁

先修复 E2E harness 的完整 observability 配置，再让现有 CI `test` job 设置唯一规范开关 `AEGISCORE_TEST_CONTAINERS=1` 并运行 `make test`。该开关同时覆盖 common 容器 smoke、role PostgreSQL 测试和 user-service E2E；不新增 `TEST_CONTAINERS` 别名，也不只设置覆盖范围较窄的 `AEGISCORE_TEST_E2E`。

## Risks / Trade-offs

- [Risk] Distroless 缺少 shell，传统 `docker exec` 诊断方式不可用。-> Mitigation：依赖结构化日志、metrics、pprof、健康端点和平台 ephemeral debug container；生产镜像不增加 debug fallback。
- [Risk] 新健康检查命令的响应判定与 `/readyz` 实际 envelope 漂移。-> Mitigation：复用稳定健康响应结构并增加成功、非 ready、非法 JSON、非 2xx、超时和连接失败测试。
- [Risk] UID 变化导致挂载目录不可写。-> Mitigation：同步更新 `fsGroup`、emptyDir 和 Helm values，并用只读根文件系统下的 `/tmp` 日志目录执行容器验证。
- [Risk] 基础镜像 digest 可能是单架构摘要，破坏多架构构建。-> Mitigation：固定审核后的多架构 manifest digest，并在 CI 验证目标架构。
- [Risk] `go mod download` 的 workspace 范围大于服务实际编译依赖，可能暴露未规范化 checksum。-> Mitigation：实现第一步规范化并审查 `go.work.sum`，之后用只读模式阻止静默更新。
- [Risk] Docker-backed 测试增加 CI 时间和 runner Docker 负载。-> Mitigation：只在一个阻塞 job 启用，保留 unit/race/coverage 的轻量路径，并维持测试容器随机名称和动态端口隔离。
- [Risk] 一次性切换镜像、UID、健康检查和 CI 可能扩大失败面。-> Mitigation：按依赖顺序先完成 CLI、测试和镜像验证，再同步部署资产，最后启用 CI 门禁；禁止混用新镜像与旧 Compose/Helm 清单。

## Migration Plan

1. 规范化 workspace checksum，完成健康检查 CLI、E2E harness 修复及对应 Go 测试。
2. 重构 Dockerfile，构建并验证 Distroless 镜像的静态链接、nonroot、timezone、CA、`/tmp`、无 shell/Atlas 和 CLI 可执行性。
3. 同步 Compose、Kubernetes、Helm 的 UID/GID 和健康检查配置，完成静态渲染与容器运行验证。
4. 合并 CI 镜像构建与安全扫描，确认同一 image ID/digest 被复用，并启用 external BuildKit cache。
5. 在现有 CI `test` job 设置 `AEGISCORE_TEST_CONTAINERS=1`，确认全部真实依赖测试执行且无 skip、E2E 通过。
6. 更新开发、测试、架构和部署文档，完成 OpenSpec、lint 和完整验证。

回滚必须按完整 release 工件集回退 Dockerfile、镜像、Compose/Kubernetes/Helm 和 CI 配置，不允许用新 Distroless 镜像搭配旧 shell healthcheck，也不允许保留 Alpine 或 UID 10001 的长期 fallback。

## Open Questions

无。
