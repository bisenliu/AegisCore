#!/usr/bin/env sh
set -eu

# 基于当前 Ent schema 生成新的 Atlas SQL 迁移文件。
#
# 用法：
#   ./scripts/migrate-diff.sh <migration-name>
#
# 示例：
#   ./scripts/migrate-diff.sh create_users
#   ./scripts/migrate-diff.sh add_user_status
#   ./scripts/migrate-diff.sh add_user_email_index
#
# 可选环境变量：
#   ATLAS_DEV_URL  migrations/atlas.hcl 使用的 Atlas dev database URL。默认使用
#                  migrations/atlas.hcl 中的值，当前使用本仓库 latest pg_trgm 镜像；
#                  正式/受控环境建议固定具体 PostgreSQL 版本或镜像 digest。
#
# 行为：
#   - 切换到 user-service 目录，使 Atlas 读取 ./migrations/atlas.hcl 和 ./migrations。
#   - 确保 Atlas dev database 镜像预置 pg_trgm，支持 Ent schema 中的 gin_trgm_ops。
#   - 使用 Atlas external schema loader 作为期望 schema 来源，并全局禁用数据库真实外键。
#   - 先重新计算已有迁移目录 hash，避免人工拆分或删除 SQL 后 diff 前校验失败。
#   - 在 dev database 上回放已有迁移，将结果与 Ent schema 对比；如有差异，
#     在 user-service/migrations/ 下写入新的 SQL 文件。
#   - 生成完成后重新计算 user-service/migrations/atlas.sum。
#
# Review 流程：
#   1. 提交前审查生成的 .sql 文件。
#   2. 如果手动修改 SQL，必须再次运行本脚本或 `atlas migrate hash --dir file://migrations`。
#   3. 提交前或 CI 中运行 `./scripts/migrate-validate.sh`。
if [ "$#" -ne 1 ]; then
  echo "用法：$0 <migration-name>" >&2
  exit 2
fi

cd "$(dirname "$0")/.."

# 本地迁移 diff 默认跟随最新 PostgreSQL；正式环境建议固定具体版本或镜像 digest，避免生成结果漂移。
ATLAS_DEV_IMAGE="aegiscore-atlas-postgres-pgtrgm:latest"
ATLAS_DEV_DOCKERFILE="../deployments/docker/atlas-postgres-pgtrgm.Dockerfile"
docker build -f "$ATLAS_DEV_DOCKERFILE" -t "$ATLAS_DEV_IMAGE" ..

# migrate diff 会先校验 migration directory；人工拆分、删除或编辑 SQL 后需要先刷新 hash。
atlas migrate hash --dir file://migrations

# Atlas 通过 external schema loader 读取 Ent schema；GOWORK=off 用于避免 workspace 模式冲突。
GOWORK=off atlas migrate diff "$1" --config file://migrations/atlas.hcl --env local

# 生成迁移或人工审查并修改 SQL 后，重新计算 atlas.sum。
atlas migrate hash --dir file://migrations
