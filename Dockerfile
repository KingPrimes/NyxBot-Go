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

COPY dist/linux/${TARGETARCH}/NyxBot /app/NyxBot
COPY resources /app/resources
COPY config.yaml /app/config.yaml
COPY build/icons/icon.png /app/resources/icon.png
COPY build/icons/icon.svg /app/resources/icon.svg

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
