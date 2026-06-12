#!/usr/bin/env sh
set -eu

# 将已提交的 Atlas SQL 迁移应用到目标用户数据库。
#
# 用法：
#   DATABASE_URL='<postgres-url>' ./scripts/migrate-apply.sh
#
# 示例：
#   DATABASE_URL='postgres://aegiscore:aegiscore@127.0.0.1:5432/aegiscore_user?sslmode=disable&search_path=public' \
#     ./scripts/migrate-apply.sh
#
# 必需环境变量：
#   DATABASE_URL  用户服务数据库（`postgres.user_db`）的 PostgreSQL URL。
#
# 行为：
#   - 切换到 user-service 目录，使 Atlas 读取 ./migrations/atlas.hcl 和 ./migrations。
#   - 使用 migrations/atlas.hcl 中的 `deploy` 环境。
#   - 只执行 user-service/migrations/ 中已经提交的 SQL 文件。
#   - DATABASE_URL 缺失或 Atlas 迁移执行失败时，以非 0 状态退出。
#
# 注意：
#   此脚本不得生成迁移，也不得调用 Ent 运行时建表逻辑。它用于 CI/CD release job，
#   或用于容器启动 HTTP 服务并开始接收流量之前。
cd "$(dirname "$0")/.."

if [ -z "${DATABASE_URL:-}" ]; then
  echo "执行 Atlas 迁移需要设置 DATABASE_URL" >&2
  exit 2
fi

atlas migrate apply --config file://migrations/atlas.hcl --env deploy
