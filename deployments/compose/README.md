# Compose 部署

本目录承载本地依赖和可选本地服务启动所需的 Docker Compose 文件。

当前状态：

- `postgres/initdb/` 预留给未来 Compose 文件使用的 PostgreSQL 初始化脚本。
- 当前没有提交可直接运行的 Compose 文件。
- PostgreSQL 和 Redis 可用后，可以通过 `make run-user-service` 直接启动用户服务。
- 从仓库根目录构建用户服务镜像：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
```

未来新增 Compose 文件时，应放在本目录下，使用仓库根目录作为 build context，并通过 `deployments/docker/user-service.Dockerfile` 构建用户服务镜像。
