## Context

本地 Compose 同时运行 PostgreSQL、Redis、Nacos、Jaeger、user-service、Prometheus 和 Grafana。当前 user-service 从 Nacos 的 `runtime.timezone` 初始化 Go `time.Local`，Nacos 基础镜像也默认使用 `Asia/Shanghai`，但其余组件主要使用 UTC。跨组件查看 Docker logs、SQL session、trace 和 dashboard 时容易把绝对时间、进程日志时区和浏览器展示时区混淆。

实测当前 `jaegertracing/all-in-one:latest` 为 Jaeger 1.76，其镜像不存在 `/usr/share/zoneinfo`，设置 `TZ=Asia/Shanghai` 后 Go 进程仍使用 UTC。Prometheus 镜像包含 IANA zoneinfo，设置相同环境变量后进程日志使用 `+08:00`。Grafana dashboard 当前显式设置为 `browser`。PostgreSQL 已有 data volume 会保留初始化时写入的 `timezone` 与 `log_timezone`，仅增加环境变量不能修正已有实例。

本变更只影响 `deployments/`、相关文档和 OpenSpec，不修改 `common`、user-service Go 代码、HTTP API、数据库 schema、Atlas migration、OpenAPI 或 RBAC policy sync。

## Goals / Non-Goals

**Goals:**

- 让本地 Compose 常驻服务和一次性任务显式声明 `Asia/Shanghai`。
- 让 Jaeger、Prometheus、Redis、Grafana 与 PostgreSQL 的进程日志可验证地使用 UTC+8。
- 让已有 PostgreSQL data volume 重启后立即使用 `Asia/Shanghai` session 与日志时区。
- 让 Grafana 默认 dashboard 使用 `Asia/Shanghai` 展示。
- 清楚记录 Prometheus UI、Jaeger UI 与 telemetry 时间戳不能由容器时区统一覆盖的边界。

**Non-Goals:**

- 不改变 OpenTelemetry span、Prometheus sample 或数据库业务时间字段的存储格式。
- 不修改 Jaeger UI 前端代码，也不伪造服务端不存在的 UI timezone 选项。
- 不迁移 Jaeger 1.x 到 Jaeger 2.x；版本升级需独立 change 评估配置与数据兼容性。
- 不改变 Kubernetes 或 Helm 的 user-service 时区契约，它们已通过 `TZ` 管理服务进程时区。
- 不删除、重建或迁移任何 Compose volume。

## Decisions

### Decision 1: Compose 使用统一 IANA 时区环境变量

Compose 定义可复用的 timezone environment anchor，并让所有常驻服务及 `nacos-init`、`rbac-seed` 一次性任务消费 `TZ=Asia/Shanghai`。这使配置可直接通过 `docker compose config` 和容器 inspect 验证，同时不依赖宿主机 `/etc/localtime` 挂载。

备选方案是挂载宿主机 `/etc/localtime`。Docker Desktop 的 Linux VM 时区不保证与 macOS 宿主机一致，而且该方式不能表达稳定 IANA 名称，因此不采用。

### Decision 2: Jaeger 使用只补 zoneinfo 的本地薄镜像

新增 Jaeger Dockerfile，以仓库已固定 digest 的 Distroless runtime 作为时区文件来源，只复制 `/usr/share/zoneinfo/Asia/Shanghai` 到当前 Jaeger 1.76 image，并设置 `TZ=Asia/Shanghai`。最终 Jaeger 入口、用户、端口和功能保持基础镜像原样。

备选方案是只设置 `TZ` 或使用 `CST-8`。前者已验证无效；后者缺少 IANA 历史规则且 Go `time` 不保证按 POSIX TZ string 加载，因此不采用。另一个备选方案是升级到 Jaeger 2.x，但会扩大配置和兼容性范围。

### Decision 3: PostgreSQL 显式覆盖 runtime timezone

PostgreSQL 同时设置 `TZ=Asia/Shanghai`，并在官方 `postgres` 命令后追加 `-c timezone=Asia/Shanghai` 与 `-c log_timezone=Asia/Shanghai`。命令行参数优先于 data volume 中已有的 `postgresql.conf`，因此新旧 volume 行为一致，且不触发 schema 或数据迁移。

备选方案是只设置 `TZ`。它只影响新 `initdb` 的推导，不能修复已有 volume，因此不采用。

### Decision 4: 区分日志、dashboard 与 telemetry 时间语义

Grafana server 设置 `TZ=Asia/Shanghai`，dashboard 源文件把 `timezone` 从 `browser` 改为 `Asia/Shanghai`，再通过现有脚本生成 Compose provisioning 副本。Prometheus process log 使用 `TZ`，但 Prometheus Web UI 继续按官方行为显示 UTC。Jaeger process 加载上海时区，但 Jaeger UI 继续使用访问浏览器的本地时区。OpenTelemetry 与 Prometheus 数据继续使用 Unix epoch 表达绝对时间。

备选方案是修改或替换 Jaeger/Prometheus 前端资产。该方案依赖非官方 patch，会增加供应链和升级负担，因此不采用。

## Risks / Trade-offs

- [Risk] 浮动 `latest` 可能改变镜像文件布局或时区支持 -> Mitigation：Compose 构建与真实容器验证检查 Jaeger zoneinfo、环境变量和进程日志；正式环境仍固定版本或 digest。
- [Risk] `CST` 缩写可能与其他地区时区混淆 -> Mitigation：配置与验证均以 IANA 名称 `Asia/Shanghai` 和数值 offset `+0800` 为准，不以缩写作为唯一判断。
- [Risk] 使用本地时间日志可能增加跨区域聚合难度 -> Mitigation：该行为仅用于本地 Compose；telemetry 存储和标准时间戳保持绝对时间语义，文档明确边界。
- [Risk] PostgreSQL command override 可能被误认为替换官方 entrypoint -> Mitigation：仅覆盖 image CMD，保留 `docker-entrypoint.sh`，并验证现有 data volume、`show timezone` 和 healthcheck。

## Migration Plan

1. 新增 Jaeger 薄镜像并更新 Compose timezone environment、PostgreSQL runtime 参数和 Grafana dashboard。
2. 运行 Compose 配置解析、dashboard drift、OpenSpec 和架构检查。
3. 使用 `docker compose up -d --build` 重建受影响容器，不删除 volume。
4. 检查所有容器的 `TZ`，验证 Jaeger/Prometheus/Redis/PostgreSQL/Grafana 启动日志或进程配置使用 UTC+8，并确认 user-service healthcheck 正常。
5. 回滚时恢复 Compose、Dockerfile、dashboard 与文档后重建容器；数据卷无需回滚。

## Open Questions

无。
