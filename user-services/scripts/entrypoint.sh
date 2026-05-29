#!/usr/bin/env sh
set -eu

# user-services 容器入口脚本。
#
# Dockerfile 中的用法：
#   ENTRYPOINT ["/app/user-services/scripts/entrypoint.sh"]
#   CMD ["/app/user-services/bin/user-services", "serve", "--config", "/app/user-services/configs/config.yaml"]
#
# 行为：
#   - RUN_MIGRATIONS 未设置或为 "true" 时，先执行 Atlas 迁移，再启动服务。
#   - RUN_MIGRATIONS 设置为其他值时，跳过迁移。
#   - 使用 CMD 或 docker run 传入的命令替换当前 shell 进程。
#
# 启用迁移时必需的环境变量：
#   DATABASE_URL='postgres://user:pass@host:5432/aegiscore_user?sslmode=require&search_path=public'
#
# 注意：
#   生产环境优先使用单独的 CI/CD release job 执行迁移。入口脚本迁移适合简单部署，
#   但多副本滚动发布可能同时启动多个容器，需要依赖 Atlas migration lock 和部署层排序。
if [ "${RUN_MIGRATIONS:-true}" = "true" ]; then
  /app/user-services/scripts/migrate-apply.sh
fi

exec "$@"
