# Docker 部署

适用：有可用的 Docker 引擎（Linux / macOS / Windows 均可）。
镜像：`kingprimes/nyxbot-go`（Docker Hub）与 `ghcr.io/<owner>/nyxbot-go`（GHCR），支持 `linux/amd64` 与 `linux/arm64`。

> 基础镜像为 `debian:12-slim`（glibc）。**2026-10-08 之前发布的镜像基于 Alpine，容器会立即退出**
> （产物是 glibc 动态链接 ELF，musl 下无法执行），拉到旧镜像请升级或自行构建。

## 1. 前置检查

```bash
docker version --format '{{.Server.Version}}'   # 有版本号 = 引擎可用
uname -m                                        # x86_64 / aarch64，无需手动选架构，镜像多架构自动匹配
```

## 2. 拉取镜像

```bash
# Docker Hub（若未配置该仓库，用下面 GHCR 那条）
docker pull kingprimes/nyxbot-go:latest
docker pull ghcr.io/<owner>/nyxbot-go:latest
```

标签规则：`latest`、`v1.2.3`、`v1.2`、`v1`（由 release 工作流按 semver 生成）。
生产环境建议钉具体版本号，便于回滚。

自检（可选）：

```bash
docker run --rm kingprimes/nyxbot-go:latest --version
# 期望：NyxBot <版本> (<commit>)；若报 “no such file or directory”，说明是旧的 Alpine 镜像，不能用
```

## 3. 首次启动：生成配置与初始凭据

镜像内不含 `config.yaml`，首次启动会在 `/app` 下生成它，并写出初始管理员凭据。
这一步**先不要挂载 config.yaml**（挂载不存在的宿主文件会被 Docker 建成目录，导致配置无法生成）。

```bash
docker volume create nyxbot-data

docker run -d --name nyxbot-init \
  -v nyxbot-data:/app/data \
  kingprimes/nyxbot-go:latest

sleep 20
docker logs nyxbot-init | grep -E "default config created|default admin user created"
# 期望看到：
#   config.yaml not found, default config created
#   default admin user created, credentials saved to admin-credentials.txt
```

取出两个文件到宿主（后续正式运行要挂载 `config.yaml`；凭据只在首次生成，之后不再覆盖）：

```bash
mkdir -p ./nyxbot && chmod 700 ./nyxbot
docker cp nyxbot-init:/app/config.yaml           ./nyxbot/config.yaml
docker cp nyxbot-init:/app/admin-credentials.txt ./nyxbot/admin-credentials.txt
docker rm -f nyxbot-init
```

> 此时 `nyxbot-data` 卷里已经有 SQLite 库与刚才创建的管理员；重建容器不会重新生成管理员。

## 4. 正式运行

```bash
docker run -d --name nyxbot \
  --restart unless-stopped \
  -p 8080:8080 \
  -v "$PWD/nyxbot/config.yaml:/app/config.yaml" \
  -v nyxbot-data:/app/data \
  kingprimes/nyxbot-go:latest
```

要点：

- `/app` **必须保持可写**（初始写 `admin-credentials.txt` 需要），不要给它加 `:ro`。
- `config.yaml` 挂载的是**宿主上非空的真实文件**（见硬约束 #3）。
- 数据卷 `nyxbot-data` 存放 `data/nyxbot.db`、`data/logs/`、`data/ocr_models/`（首次启动自动下载约 83MB）。
- 时区已固定 `Asia/Shanghai`，无需额外配置；端口冲突时改 `-p <宿主端口>:8080`。

## 5. 验收

```bash
BASE=http://localhost:8080

curl -s $BASE/api/health
# {"code":200,"msg":"success","data":{"status":"ok"}}

docker exec nyxbot cat /app/admin-credentials.txt   # 取初始用户名/密码（登录后立即改密）

curl -s -X POST $BASE/auth/login -H "Content-Type: application/json" \
  -d '{"userName":"<用户名>","password":"<密码>"}'
# {"code":200,...,"data":{"token":"eyJ...","refreshToken":"eyJ..."}}

curl -s -i $BASE/ws/shiro | head -1
# HTTP/1.1 400 Bad Request ← 反向 WS 路由已注册（200+HTML 表示路径没生效）
```

完整验收清单见 [AI 部署手册 §4](../ai-deploy.md#4-通用验收三条命令任何部署方式都适用)。

## 6. 修改 OneBot 配置

两种方式（改完**都要重启容器**，见硬约束 #5）：

```bash
# 方式 A：调接口（按非空字段增量合并，并最小化写回 config.yaml）
TOKEN=...   # 登录后拿到的 access token
curl -s -X POST http://localhost:8080/config/loading \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"isServerOrClient":true,"wsServerUrl":"/ws/shiro","token":"<OneBot 访问令牌>"}'
docker restart nyxbot

# 方式 B：直接编辑 ./nyxbot/config.yaml（bot 块：mode / ws_server_path / access_token / ws_client_url / wait_n / plugin_prefix）
docker restart nyxbot
```

重启后复核：

```bash
curl -s http://localhost:8080/config/loading -H "Authorization: Bearer $TOKEN"
# data.wsServerUrl / data.token / data.isServerOrClient 应为新值
```

然后让 OneBot 客户端（NapCat / LLOneBot 等）以反向 WS 连接 `ws://<宿主>:8080/ws/shiro`，
日志出现 `OneBot connected: <selfID>` 即为接入成功。

## 7. 升级与回滚

```bash
# 升级：拉新镜像后用同样的挂载参数重建容器（数据卷与配置文件保持不变）
docker pull kingprimes/nyxbot-go:v1.3.0
docker rm -f nyxbot
docker run -d --name nyxbot --restart unless-stopped -p 8080:8080 \
  -v "$PWD/nyxbot/config.yaml:/app/config.yaml" -v nyxbot-data:/app/data \
  kingprimes/nyxbot-go:v1.3.0
```

- 升级前建议备份：`docker run --rm -v nyxbot-data:/d -v "$PWD:/b" debian:12-slim tar czf /b/nyxbot-data.tgz -C /d .`
- 回滚只需把镜像 tag 换回旧版本；**数据库结构随版本自动迁移，降级不保证兼容**，必要时连同备份一起回退。

## 8. 自行构建镜像（可选，用于旧镜像不可用或需要自定义）

`Dockerfile` 复用 CI 产物路径 `dist/linux/<arch>/NyxBot`：

```bash
mkdir -p dist/linux/amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/linux/amd64/NyxBot ./cmd/server
DOCKER_BUILDKIT=1 docker build --build-arg TARGETARCH=amd64 -t nyxbot-go:local .
```

> 这样编译出的二进制**不含前端页面**（`resources/static` 为空时 `go:embed` 只能嵌到占位文件）；
> 需要完整 WebUI 请先按 README「托管前端」一节构建前端产物再编译。

## 9. 故障排查

| 现象 | 处置 |
|------|------|
| 容器瞬间退出，日志 `exec: /app/NyxBot: no such file or directory` | 用了旧 Alpine 镜像，升级镜像 |
| 启动 panic：`failed to initialize default admin user` | `/app` 被只读挂载或宿主文件占用，检查挂载参数 |
| 挂载 config.yaml 后报 `read config.yaml failed: is a directory` | 宿主 `config.yaml` 不存在，Docker 建成了目录；删除该目录并按 §3 生成 |
| 日志 `OCR 准备失败` | 出网受限，模型（83MB）下载失败；服务不受影响。可在有网环境下载 det/rec.onnx 后放入卷内 `data/ocr_models/` |
| 页面打不开但 API 正常 | 端口映射不对；`-p` 的宿主端口与访问端口要一致 |
