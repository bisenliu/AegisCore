# 使用示例：
# docker build \
#   --build-arg NACOS_VERSION=latest \
#   -f deployments/docker/nacos.Dockerfile \
#   -t aegiscore-nacos:latest \
#   .
#
# 参数说明：
# - NACOS_VERSION：Nacos 基础镜像版本；latest 仅用于本地验证，生产环境应固定版本或镜像 digest。
# - NACOS_AUTH_ENABLE：是否开启 SDK/gRPC Client 请求鉴权；本地 Compose 默认开启。
# - NACOS_AUTH_ADMIN_ENABLE：是否开启 /v3/admin/* Admin API 鉴权；本地 Compose 默认开启。
# - NACOS_AUTH_CONSOLE_ENABLE：是否开启 /v3/console/* Console API 和登录鉴权；本地 Compose 默认开启。
# - NACOS_AUTH_TOKEN：默认鉴权插件的 JWT Base64 签名密钥；示例值仅限本地，生产环境必须替换。
# - NACOS_AUTH_IDENTITY_KEY/VALUE：Nacos 节点间请求的身份标识键值；生产环境必须替换并妥善保管。
# - MODE/FUNCTION_MODE：镜像内已固定为 standalone/microservice，docker run 无需重复传入。
#
# docker run -d --name aegiscore-nacos \
#   -p 127.0.0.1:8849:8080 \
#   -p 127.0.0.1:8848:8848 \
#   -p 127.0.0.1:9848:9848 \
#   -e NACOS_AUTH_ENABLE=true \
#   -e NACOS_AUTH_ADMIN_ENABLE=true \
#   -e NACOS_AUTH_CONSOLE_ENABLE=true \
#   -e NACOS_AUTH_TOKEN=QWVnaXNDb3JlTmFjb3NMb2NhbERldmVsb3BtZW50VG9rZW4wMTIzNDU2Nzg5 \
#   -e NACOS_AUTH_IDENTITY_KEY=aegiscore-local \
#   -e NACOS_AUTH_IDENTITY_VALUE=aegiscore-local \
#   -v aegiscore-nacos-data:/home/nacos/data \
#   aegiscore-nacos:latest

ARG NACOS_VERSION=latest
FROM nacos/nacos-server:${NACOS_VERSION}

# docker-startup.sh 将以下变量转换为 -Dnacos.standalone=true 和 -Dnacos.functionMode=microservice。
ENV MODE=standalone \
    FUNCTION_MODE=microservice

EXPOSE 8080 8848 9848

# 直接运行镜像时使用官方容器启动脚本，不依赖 Compose command。
ENTRYPOINT ["bash", "bin/docker-startup.sh"]
