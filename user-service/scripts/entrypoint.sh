#!/usr/bin/env sh
set -eu

# user-service 容器入口脚本。
#
# Dockerfile 中的用法：
#   ENTRYPOINT ["/app/user-service/scripts/entrypoint.sh"]
#   CMD ["/app/user-service/bin/user-services", "serve", "--config", "/app/user-service/configs/config.yaml"]
#
# 行为：
#   - RUN_MIGRATIONS 设置为 "true" 时，先执行 Atlas 迁移，再启动服务。
#   - RUN_MIGRATIONS 未设置或设置为其他值时，跳过迁移。
#   - 使用 CMD 或 docker run 传入的命令替换当前 shell 进程。
#
# 启用迁移时必需的环境变量：
#   DATABASE_URL='postgres://user:pass@host:5432/aegiscore_user?sslmode=require&search_path=public'
#
# 注意：
#   生产环境优先使用单独的 CI/CD release job 或 migration Job 执行迁移。入口脚本迁移只
#   适合简单部署或兼容场景；多副本滚动发布不应让普通服务副本竞争 Atlas migration lock。
if [ "${RUN_MIGRATIONS:-false}" = "true" ]; then
  /app/user-service/scripts/migrate-apply.sh
fi

exec "$@"
