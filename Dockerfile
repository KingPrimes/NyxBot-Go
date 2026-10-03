# 运行时镜像：直接使用 CI linux job 构建好的二进制（dist/linux/{TARGETARCH}/NyxBot）
# 不再在 Docker 内重复编译；多平台构建由 buildx 的 TARGETARCH 自动选择对应产物
FROM alpine:3.20

# buildx 多平台构建时自动注入（linux/amd64 → amd64，linux/arm64 → arm64）
ARG TARGETARCH

RUN apk add --no-cache ca-certificates tzdata font-noto-cjk \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && rm -rf /var/cache/apk/*

WORKDIR /app

# 前端已由 go:embed 打进二进制（见 resources/assets.go），镜像内无需再放 resources 目录
COPY dist/linux/${TARGETARCH}/NyxBot /app/NyxBot
COPY config.yaml /app/config.yaml

ARG VERSION=dev
ARG PRODUCT_NAME=NyxBot
ARG DESCRIPTION="NyxBot Server"
ARG VCS_REF=unknown

LABEL org.opencontainers.image.title="$PRODUCT_NAME"
LABEL org.opencontainers.image.description="$DESCRIPTION"
LABEL org.opencontainers.image.version="$VERSION"
LABEL org.opencontainers.image.revision="$VCS_REF"
LABEL org.opencontainers.image.vendor="KingPrimes"

EXPOSE 8080

ENTRYPOINT ["/app/NyxBot"]
