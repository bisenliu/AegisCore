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
#   ATLAS_DEV_URL  atlas.hcl 使用的 Atlas dev database URL。默认使用 atlas.hcl 中的值，
#                  通常是 docker://postgres/15/dev?search_path=public。
#
# 行为：
#   - 切换到 user-service 目录，使 Atlas 读取 ./atlas.hcl 和 ./migrations。
#   - 使用 Atlas `ent://ent/schema` 作为期望 schema 来源。
#   - 在 dev database 上回放已有迁移，将结果与 Ent schema 对比；如有差异，
#     在 user-service/migrations/ 下写入新的 SQL 文件。
#   - 生成完成后重新计算 user-service/migrations/atlas.sum。
#
# Review 流程：
#   1. 提交前审查生成的 .sql 文件。
#   2. 如果手动修改 SQL，必须再次运行 `atlas migrate hash --dir file://migrations`。
#   3. 提交前或 CI 中运行 `./scripts/migrate-validate.sh`。
if [ "$#" -ne 1 ]; then
  echo "用法：$0 <migration-name>" >&2
  exit 2
fi

cd "$(dirname "$0")/.."

# Atlas 读取 ent:// schema source 时会以 -mod=mod 调用 Go；GOWORK=off 用于避免 workspace 模式冲突。
GOWORK=off atlas migrate diff "$1" --env local

# 生成迁移或人工审查并修改 SQL 后，重新计算 atlas.sum。
atlas migrate hash --dir file://migrations
