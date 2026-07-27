# Nacos 本地配置

本目录是本地 Nacos 配置的 Git 权威来源。主机进程和 Docker Compose 使用独立 Namespace，各自目录中的三份 YAML 都是可直接发布的完整配置，不需要 overlay 或 target manifest。

## 目录

```text
deployments/nacos/
├── local-host/
│   ├── base.yaml
│   ├── resources.yaml
│   └── user-service.yaml
└── local-docker/
    ├── base.yaml
    ├── resources.yaml
    └── user-service.yaml
```

| 配置目录 | Nacos Namespace | 连接地址 |
|---|---|---|
| `local-host/` | `loca-host` | PostgreSQL、Redis 和 OTLP 使用宿主机映射端口 |
| `local-docker/` | `loca-docker` | PostgreSQL、Redis 和 OTLP 使用 Compose service DNS |

两个目录都发布并加载 `base.yaml,resources.yaml,user-service.yaml`。公共字段会重复保存；修改公共配置时必须同步修改两个目录，测试会检查公共字段保持一致，只允许环境连接地址存在差异。

## 发布

Compose 使用两个一次性初始化任务分别发布两个目录。也可以在仓库根目录手动发布：

```bash
go run ./tools/nacos-config-seed \
  --addr 127.0.0.1:8848 \
  --namespace loca-host \
  --config-dir ./deployments/nacos/local-host

go run ./tools/nacos-config-seed \
  --addr 127.0.0.1:8848 \
  --namespace loca-docker \
  --config-dir ./deployments/nacos/local-docker
```

相同命令可幂等重跑并覆盖已有 dataId。工具不会自动删除旧 dataId 或旧 Namespace；网络阶段中途失败时，修复问题后重新运行对应命令。

当前 user-service 不动态监听 Nacos 变化，重新发布后需要重启或滚动对应进程。不要把 Nacos 控制台手工修改当作配置来源。目录内的固定 secret 只用于隔离的本地开发环境，production-like secret 不得提交到 Git。

## 从旧 Namespace 迁移

1. 先分别发布 `local-host/` 与 `local-docker/`，并在 `loca-host`、`loca-docker` 执行 `config sources`、`config validate` 和脱敏后的 `config render`。
2. 主机进程切换到 `loca-host`，Compose workload 切换到 `loca-docker`，重启后确认依赖连接和健康检查正常。
3. 需要回滚时，显式把对应进程的 `AEGISCORE_NACOS_NAMESPACE` 改回旧 `loca` 并重启；不要依赖自动回退。
4. 确认没有进程继续使用旧 `loca` 后，再通过 Nacos 管理流程人工清理。seed 工具不会删除旧 Namespace 或其中的旧 dataId。
