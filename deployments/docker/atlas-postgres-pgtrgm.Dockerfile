# 从仓库根目录构建：
#   docker build -f deployments/docker/atlas-postgres-pgtrgm.Dockerfile -t aegiscore-atlas-postgres-pgtrgm:15 .

FROM postgres:15

RUN printf '%s\n' 'CREATE EXTENSION IF NOT EXISTS pg_trgm;' > /docker-entrypoint-initdb.d/001_pg_trgm.sql
