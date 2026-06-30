# 从仓库根目录构建：
#   docker build -f deployments/docker/user-service-migration.Dockerfile -t aegiscore-user-services-migration .

FROM arigaio/atlas:latest

WORKDIR /app/user-service

COPY user-service/migrations /app/user-service/migrations

ENTRYPOINT ["/atlas"]
CMD ["migrate", "apply", "--config", "file://migrations/atlas.hcl", "--env", "deploy"]
