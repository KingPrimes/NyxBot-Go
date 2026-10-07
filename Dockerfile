# 运行时镜像：直接使用 CI linux job 构建好的二进制（dist/linux/{TARGETARCH}/NyxBot）
# 不再在 Docker 内重复编译；多平台构建由 buildx 的 TARGETARCH 自动选择对应产物
#
# 基础镜像必须是 glibc 发行版（这里是 Debian slim），不能用 Alpine（musl）：
# purego 通过 //go:cgo_import_dynamic 链接 libdl.so.2 / libc.so.6（见
# github.com/ebitengine/purego dlfcn_nocgo_linux.go），即使 CGO_ENABLED=0 产物也是
# 动态链接 ELF，在 musl 上会以 "exec: no such file or directory" 直接起不来
# （装 libc6-compat 也无效，实测）；内嵌的 onnxruntime 动态库同为 glibc 链接
# （internal/ocr/assets/lib），musl 下即便能启动也加载不了。
FROM debian:12-slim

# buildx 多平台构建时自动注入（linux/amd64 → amd64，linux/arm64 → arm64）
ARG TARGETARCH

# ca-certificates：OCR 模型与 Warframe 数据的 https 下载；fonts-noto-cjk：中文绘图
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata fonts-noto-cjk \
    && ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 前端已由 go:embed 打进二进制（见 resources/assets.go），镜像内无需再放 resources 目录
# config.yaml 也不入镜像：它含 JWT 密钥、属运行期配置，容器首次启动会用默认值在 /app 下生成
COPY dist/linux/${TARGETARCH}/NyxBot /app/NyxBot

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
