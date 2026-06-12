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
  -g cmd/main.go \
  -o docs \
  --packagePrefix github.com/aegiscore/user-service \
  --useStructName \
  --parseDependency \
  --parseInternal
