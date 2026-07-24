FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags "-X nyxbot-go/internal/version.Version=${VERSION} -X nyxbot-go/internal/version.Commit=${COMMIT}" \
    -o /out/NyxBot ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata font-noto-cjk \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=builder /out/NyxBot /app/NyxBot
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
