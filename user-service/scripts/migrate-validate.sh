#!/usr/bin/env sh
set -eu

# 校验 Atlas 迁移目录完整性。
#
# 用法：
#   ./scripts/migrate-validate.sh
#
# 行为：
#   - 切换到 user-service 目录。
#   - 使用 user-service/migrations/atlas.sum 校验 user-service/migrations/。
#   - 如果迁移文件被新增、删除、重排或编辑，但没有重新计算 atlas.sum，则以非 0 状态退出。
#
# 如果人工修改 SQL 后校验失败：
#   atlas migrate hash --dir file://migrations
#   ./scripts/migrate-validate.sh
#
# 推荐用法：
#   在 CI 中、Docker 构建前或 `migrate-apply.sh` 执行前运行，避免部署损坏的迁移目录。
cd "$(dirname "$0")/.."

atlas migrate validate --dir file://migrations
