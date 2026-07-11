# 从仓库根目录构建：
#   docker build -f deployments/docker/atlas-postgres-pgtrgm.Dockerfile -t aegiscore-atlas-postgres-pgtrgm:latest .

# 本地迁移 diff/校验镜像跟随最新 PostgreSQL；正式环境建议固定具体版本或镜像 digest，避免生成结果随上游镜像漂移。
FROM postgres:latest

RUN printf '%s\n' 'CREATE EXTENSION IF NOT EXISTS pg_trgm;' > /docker-entrypoint-initdb.d/001_pg_trgm.sql
