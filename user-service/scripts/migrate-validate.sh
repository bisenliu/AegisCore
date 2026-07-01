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
#   - 校验对象是 migration directory 本身；不需要读取 migrations/atlas.hcl。
#   - 如果迁移文件被新增、删除、重排或编辑，但没有重新计算 atlas.sum，则以非 0 状态退出。
#
# 如果人工修改 SQL 后校验失败：
#   atlas migrate hash --dir file://migrations
#   ./scripts/migrate-validate.sh
#
# 推荐用法：
#   在 CI、SQL 工单提交前或受控发布平台执行前运行，避免交付损坏的迁移目录。
cd "$(dirname "$0")/.."

atlas migrate validate --dir file://migrations
