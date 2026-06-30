#!/usr/bin/env sh
set -eu

# user-service 容器入口脚本。
#
# Dockerfile 中的用法：
#   ENTRYPOINT ["/app/user-service/scripts/entrypoint.sh"]
#   CMD ["/app/user-service/bin/user-services", "serve", "--config", "/app/user-service/configs/config.yaml"]
#
# 行为：
#   - 普通 user-service 运行时镜像不包含 Atlas，不执行数据库迁移。
#   - RUN_MIGRATIONS 设置为 "true" 时立即失败，避免误以为服务镜像会执行迁移。
#   - 使用 CMD 或 docker run 传入的命令替换当前 shell 进程。
#
# 注意：
#   数据库迁移必须先通过专用 Atlas/migration 镜像或 CI/CD release job 执行，成功后再
#   启动本运行时镜像。
if [ "${RUN_MIGRATIONS:-false}" = "true" ]; then
  echo "RUN_MIGRATIONS=true 已废弃；请先运行专用 migration 镜像执行 Atlas 迁移" >&2
  exit 2
fi

exec "$@"
