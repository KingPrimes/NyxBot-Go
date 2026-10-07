# Docker 部署

适用：有可用的 Docker 引擎（Linux / macOS / Windows 均可）。
镜像：`kingprimes/nyxbot-go`（Docker Hub）与 `ghcr.io/<owner>/nyxbot-go`（GHCR，`<owner>` 换成实际用户名，
例如 `ghcr.io/kingprimes/nyxbot-go`），支持 `linux/amd64` 与 `linux/arm64`。

> **基础镜像为 `debian:12-slim`（glibc）**。2026-10-08 之前发布的镜像基于 Alpine，容器会立即退出
> （产物是 glibc 动态链接 ELF，musl 下无法执行），拉到旧镜像请升级或自行构建。
>
> **公网部署前先读**：服务本身只监听明文 HTTP/WS（`cmd/server/main.go` 用 `ListenAndServe`，不带 TLS）。
> 管理凭据、token 与 OneBot `access_token` 都会明文过网，**不要把 8080 直接暴露到公网**；
> 远程访问请放在终止 TLS 的反向代理后（HTTPS / `wss://`），详见 [手册的安全说明](../ai-deploy.md#5-安全说明公网部署必读)。

## 1. 前置检查

```bash
docker version --format '{{.Server.Version}}'   # 有版本号 = 引擎可用
uname -m                                        # x86_64 / aarch64，无需手动选架构，镜像多架构自动匹配
```

## 2. 拉取镜像

```bash
docker pull kingprimes/nyxbot-go:latest
# 或 GHCR（把 owner 换成实际用户名）
# docker pull ghcr.io/kingprimes/nyxbot-go:latest
```

标签规则：`latest`、`v1.2.3`、`v1.2`、`v1`（由 release 工作流按 semver 生成）。
生产环境建议钉具体版本号，便于回滚。

自检（可选）：

```bash
docker run --rm kingprimes/nyxbot-go:latest --version
# 期望：NyxBot <版本> (<commit>)
# 若报 “no such file or directory”，说明是旧的 Alpine 镜像，不能用
```

> `latest`（以及 `releases/latest/download`）解析到的是**最新已发布**的版本；若新版本还是**草稿**，
> 这里会静默拿到上一个已发布版本。要求精确版本时请显式指定 tag，并用 `--version` 复核。

## 3. 首次启动与正式运行（同一条命令）

程序在**工作目录**下生成 `config.yaml` 与 `data/`，所以把宿主目录挂到容器内固定路径并把**工作目录切过去**：

```bash
mkdir -p ./nyxbot && chmod 700 ./nyxbot

docker run -d --name nyxbot \
  --restart unless-stopped \
  -p 8080:8080 \
  -v "$PWD/nyxbot:/work" \
  -w /work \
  kingprimes/nyxbot-go:latest
```

- 首启会生成 `/work/config.yaml` 与 `/work/data/`（SQLite、日志、OCR 模型 ~83MB）。
- 初始管理员凭据写在**可执行文件同级**的 `/app/admin-credentials.txt`，**不在挂载目录里**，
  容器重建后就没了——立即取出并保存：

  ```bash
  sleep 15
  docker exec nyxbot cat /app/admin-credentials.txt > ./nyxbot/admin-credentials.txt
  chmod 600 ./nyxbot/admin-credentials.txt
  test -s ./nyxbot/admin-credentials.txt && head -1 ./nyxbot/admin-credentials.txt
  # 期望：NyxBot initial administrator credentials
  # 库里已有管理员后不会再生成该文件（internal/database/admin.go: ensureDefaultAdmin），
  # 拿不到明文初始密码就只能删库重来，所以这一步务必成功后再继续。
  ```

- 容器内 `/app` 必须保持可写（首启要写上面的凭据文件），不要给它加 `:ro`。
- 时区已固定 `Asia/Shanghai`；端口冲突时改 `-p <宿主端口>:8080`。

### 为什么是「目录挂载 + `-w`」

不要改回「单独挂载一个 `config.yaml` 文件」的写法：`Config.Runtime.Update` 保存配置用的是
「同目录临时文件 + `os.Rename`」（`internal/config/writeFileAtomic`），而**在单文件 bind mount 上改名
会失败**。实测（`debian:12-slim` + 官方产物）：

```text
POST /config/loading → {"code":500,"msg":"配置保存失败: rename ./config.yaml.*.tmp config.yaml: device or resource busy"}
```

宿主文件不会更新，接口改配置的能力等于失效。目录挂载没有这个问题（同一文件系统内改名），
实测改配置返回 `{"code":200,"msg":"success"}`、宿主文件即时更新、重启后仍生效。

用命名卷（`-v nyxbot:/work`）也可以，但取回 `config.yaml`/凭据需要临时容器，不如宿主目录直观。

## 4. 验收

```bash
BASE=http://localhost:8080

curl -s "$BASE/api/health"
# {"code":200,"msg":"success","data":{"status":"ok"}}

curl -s -X POST "$BASE/auth/login" -H "Content-Type: application/json" \
  -d '{"userName":"<用户名>","password":"<密码>"}'
# {"code":200,...,"data":{"token":"eyJ...","refreshToken":"eyJ..."}}

curl -s -i "$BASE/ws/shiro" | head -1
# HTTP/1.1 400 Bad Request ← 反向 WS 路由已注册（200+HTML 表示路径没生效）
```

用户名/密码从 `./nyxbot/admin-credentials.txt`（上一步保存的宿主副本）读取。
完整验收清单见 [AI 部署手册 §4](../ai-deploy.md#4-通用验收任何部署方式都适用)。

## 5. 修改 OneBot 配置

```bash
TOKEN=<登录拿到的 access token>

# 方式 A：调接口（按非空字段增量合并，最小化写回 config.yaml；目录挂载下可正常落盘）
curl -s -X POST "$BASE/config/loading" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"isServerOrClient":true,"wsServerUrl":"/ws/shiro","token":"<OneBot 访问令牌>"}'

# 方式 B：直接编辑宿主上的 ./nyxbot/config.yaml（bot 块）
```

改完**都要重启容器**（端口 / 连接模式 / WS 路径 / 令牌需重启才生效）：

```bash
docker restart nyxbot
curl -s "$BASE/config/loading" -H "Authorization: Bearer $TOKEN"
# data.wsServerUrl / data.token / data.isServerOrClient 应为新值
```

然后让 OneBot 客户端（NapCat / LLOneBot 等）以反向 WS 连接 `ws://<宿主>:8080/ws/shiro`
（配置了令牌则附加 `?access_token=<令牌>`；**跨不可信网络请走 `wss://` + TLS 代理**），
日志出现 `OneBot connected: <selfID>` 即为接入成功。

## 6. 升级与回滚

```bash
# 备份：先停容器，避免对运行中的 SQLite 做非一致性拷贝
docker stop nyxbot
docker run --rm -v "$PWD/nyxbot:/src:ro" -v "$PWD:/backup" debian:12-slim \
  tar czf /backup/nyxbot-$(date +%F).tgz -C /src .

# 升级：拉新镜像后用同样的挂载参数重建容器（/work 目录保持不变）
docker pull kingprimes/nyxbot-go:v1.3.0
docker rm -f nyxbot
docker run -d --name nyxbot --restart unless-stopped -p 8080:8080 \
  -v "$PWD/nyxbot:/work" -w /work \
  kingprimes/nyxbot-go:v1.3.0
```

- 回滚只需把 tag 换回旧版本；**数据库结构随版本自动迁移，降级不保证兼容**，必要时连备份一起回退。
- 恢复备份前先 `docker stop nyxbot`，解包后再启动。

## 7. 自行构建镜像（可选，用于旧镜像不可用或需要自定义）

`Dockerfile` 复用 CI 产物路径 `dist/linux/<arch>/NyxBot`：

```bash
mkdir -p dist/linux/amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/linux/amd64/NyxBot ./cmd/server
DOCKER_BUILDKIT=1 docker build --build-arg TARGETARCH=amd64 -t nyxbot-go:local .
```

> 这样编译出的二进制**不含前端页面**（`resources/static` 为空时 `go:embed` 只能嵌到占位文件）；
> 需要完整 WebUI 请先按 README「托管前端」一节构建前端产物再编译。

## 8. 故障排查

| 现象 | 处置 |
|------|------|
| 容器瞬间退出，日志 `exec: /app/NyxBot: no such file or directory` | 用了旧 Alpine 镜像，升级镜像 |
| 启动 panic：`failed to initialize default admin user` | `/app` 被只读挂载或目录不可写，检查挂载参数 |
| 改配置返回 `rename ... device or resource busy` | 又把 `config.yaml` 单独挂载成文件了，改回「目录挂载 + `-w`」 |
| `docker exec nyxbot cat /app/admin-credentials.txt` 报文件不存在 | 库中已有管理员（凭据只在首次生成）或容器已被重建；改读宿主保存的副本 |
| 日志 `OCR 准备失败` | 出网受限，模型（83MB）下载失败；服务不受影响。可在有网环境下载 det/rec.onnx 后放入 `/work/data/ocr_models/` |
| 页面打不开但 API 正常 | 端口映射不对；`-p` 的宿主端口与访问端口要一致 |
