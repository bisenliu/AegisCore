#!/usr/bin/env sh
set -eu

# 基于源码注解生成用户服务 Swagger/OpenAPI 文档。
#
# 用法：
#   make swagger-generate
#   cd user-service && ./scripts/swagger-generate.sh
#   ./scripts/swagger-generate.sh
#
# 生成产物：
#   user-service/docs/docs.go
#   user-service/docs/swagger.json
#   user-service/docs/swagger.yaml

cd "$(dirname "$0")/.."

go run github.com/swaggo/swag/cmd/swag@v1.16.6 init \
  -d ./cmd,./internal/router,./internal/features/auth/transport/http,./internal/features/user/transport/http,./internal/features/role/transport/http,./internal/features/permission/transport/http \
  -g main.go \
  -o docs \
  --useStructName \
  --parseDependency \
  --parseInternal
