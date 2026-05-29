#!/usr/bin/env sh
set -eu

# 基于源码注解生成用户服务 Swagger/OpenAPI 文档。
#
# 用法：
#   ./scripts/swagger-generate.sh
#
# 生成产物：
#   user-services/docs/docs.go
#   user-services/docs/swagger.json
#   user-services/docs/swagger.yaml

cd "$(dirname "$0")/.."

go run github.com/swaggo/swag/cmd/swag@v1.16.6 init \
  -g cmd/main.go \
  -o docs \
  --parseDependency \
  --parseInternal
