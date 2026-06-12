# Kubernetes 部署

本目录承载 AegisCore 服务的 Kubernetes YAML。

当前状态：

- 当前没有提交可直接运行的 Kubernetes 清单。
- `user-services/` 预留给未来用户服务清单。

未来新增清单时，应使用由 `deployments/docker/user-service.Dockerfile` 构建的镜像。除非单独变更明确设计云厂商依赖，否则不要新增云厂商特定资源。
