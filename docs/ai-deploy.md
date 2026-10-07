# NyxBot-Go AI 自动部署手册（总入口）

面向 **AI Agent**（Claude Code / Cursor / 自动化脚本）与按同样步骤操作的人：
目标是让执行者**不需要读源码**就能把 NyxBot-Go 后端部署到「服务可用 → 管理员可登录 → OneBot 可接入」，
每一步都给出命令、**可判定的期望输出**与失败处置。

> 本文事实来自仓库代码与 2026-10-08 的实测（Docker 实测于 `debian:12-slim` 容器 + 官方产物二进制）；
> 标注「需真实 OneBot 客户端」的步骤无法脱离客户端验证，请如实告知用户而不是假装通过。
>
> 命令块里的 `<占位符>` 是给人看的，**不要整段照抄**——先把它们赋成变量再用（下文示例已按此写法）。

## 0. 硬约束（先读，违反会导致部署失败）

| # | 约束 | 说明 |
|---|------|------|
| 1 | **Linux 产物需要 glibc 发行版** | 二进制是动态链接（依赖 `libdl.so.2`/`libc.so.6`，源于 purego 的 `dlopen` 绑定），**Alpine 等 musl 系统跑不起来**（实测报 `exec: no such file or directory`，装 `libc6-compat` 也无效）。官方 Docker 镜像已改为 `debian:12-slim`，用镜像部署不受此限。 |
| 2 | **可执行文件所在目录首次启动必须可写** | 库中无用户时会写 `admin-credentials.txt` 到可执行文件同级目录；写入失败会 **panic 启动失败**。不要把该目录以只读方式挂载（Docker 下即 `/app`）。 |
| 3 | **不要用空的 `config.yaml` 占位** | 实测：空文件不会被回填任何默认值，且 JWT 密钥只驻留内存（重启后旧 token 全失效）。让程序自己生成，或直接编辑程序生成的文件。 |
| 4 | **Docker 下不要单独挂载 `config.yaml` 文件** | 保存配置走「临时文件 + `os.Rename`」，单文件 bind mount 上改名会 `device or resource busy`（实测），接口改配置直接失效。请用「目录挂载 + `-w`」，见 [docker.md](deploy/docker.md)。 |
| 5 | **前端已内嵌** | 产物自带 WebUI，运行目录**不需要** `resources/`；反之，本机自行编译的产物页面来自编译时的 `resources/`，没跑过 `pnpm build` 就没有页面。 |
| 6 | **部分配置改完必须重启** | `server.port` / `bot.mode` / `bot.ws_server_path` / `bot.access_token` 改动后需重启进程才生效；`plugin_prefix` 即时生效。 |
| 7 | **首次启动会联网下载 OCR 模型** | 默认 `ocr.enabled=true`，后台下载约 83MB（det+rec.onnx）到 `data/ocr_models`；失败**不影响服务启动**，仅紫卡 OCR 指令不可用。 |
| 8 | **运行期生成物不是部署输入** | `config.yaml`、`data/`、`admin-credentials.txt` 均为运行期生成（含密钥与密码），不要提交到版本库、不要塞进镜像。 |

## 1. 分发产物对照表

| 产物 | 来源 | 适用 |
|------|------|------|
| `kingprimes/nyxbot-go:<tag>`、`ghcr.io/<owner>/nyxbot-go:<tag>` | Docker Hub / GHCR | **首选**，Linux amd64 / arm64 |
| `NyxBot-linux-amd64`、`NyxBot-linux-arm64` | GitHub Release 附件 | glibc 发行版裸机 |
| `NyxBot-darwin-amd64`、`NyxBot-darwin-arm64` | GitHub Release 附件 | macOS |
| `NyxBot.exe` | GitHub Release 附件 | Windows amd64 |
| `SHA256SUMS.txt` | GitHub Release 附件 | 校验用 |

**关于版本的两点**：

- Release 是**草稿制**：tag 触发 CI 后先建草稿，人工审核发布后才可下载。
- `releases/latest/download` 解析到的是**最新已发布**版本；新版本仍是草稿时它会**静默返回上一个已发布版本**
  （不是 404）。要求精确版本请用 `https://github.com/KingPrimes/NyxBot-Go/releases/download/<tag>/<文件>`，
  并在部署后用 `./NyxBot --version` 复核实际版本。

## 2. 环境探测（部署前先做）

```bash
uname -s && uname -m            # Linux/Darwin + x86_64(→amd64) / aarch64(→arm64)
cat /etc/os-release             # ID=alpine → 禁止走二进制路径，改用 Docker
docker version --format '{{.Server.Version}}'   # 有输出 = 可用 Docker
ss -ltn 2>/dev/null | grep ':8080 ' || echo "8080 空闲"   # 端口占用
```

## 3. 选择部署路径

```
能用 Docker（推荐） ──► docs/deploy/docker.md
         │
         └─ 不能用 Docker
              ├─ Linux（glibc，非 Alpine）──► docs/deploy/linux.md
              └─ Windows ──────────────────► docs/deploy/windows.md
```

## 4. 通用验收（三条命令，任何部署方式都适用）

先设好变量（`<host>`、`<用户名>`、`<密码>` 换成实际值；Docker 部署的用户名/密码取自
首启时保存的 `admin-credentials.txt` 宿主副本）：

```bash
BASE=http://localhost:8080          # 远程访问请改用 https://…（见 §5）
USER=<用户名>
PASS=<密码>

# ① 服务活着（成功码 200）
curl -s "$BASE/api/health"
# 期望：{"code":200,"msg":"success","data":{"status":"ok"}}

# ② 管理员可登录
curl -s -X POST "$BASE/auth/login" -H "Content-Type: application/json" \
  -d "{\"userName\":\"$USER\",\"password\":\"$PASS\"}"
# 期望：{"code":200,...,"data":{"token":"eyJ...","refreshToken":"eyJ..."}}
```

```bash
# ③ 反向 WS 路由已注册（默认路径 /ws/shiro）
curl -s -i "$BASE/ws/shiro" | head -1
# 期望：HTTP/1.1 400 Bad Request   ← 非 WebSocket 升级请求被拒 = 路由在
# 若返回 HTTP/1.1 200 且正文是 HTML（SPA 兜底），说明该路径没有生效：
#   检查 config.yaml 的 bot.ws_server_path 与 bot.mode=server，然后重启进程
# 若配置了访问令牌：未带令牌 401、令牌错误 403（令牌可经 ?access_token= 或 Authorization 头传递）
```

**端到端验收（需真实 OneBot 客户端，如 NapCat / LLOneBot）：**

1. 让客户端以**反向 WS** 连接 `ws://<host>:8080/ws/shiro`（配置了令牌则附加 `?access_token=<token>`；
   跨不可信网络请改用 `wss://`，见 §5）；
2. 后端日志出现 `OneBot connected: <selfID>`（pack=`onebot.ws`）；
3. `GET /config/bot/bots`（需管理员 token）返回机器人列表——在此之前它固定返回
   `{"code":500,"msg":"请链接机器人后操作！\n注意：官方机器人无法获取好友与群列表。"}`。

## 5. 安全说明（公网部署必读）

服务端**只监听明文 HTTP/WS**（`cmd/server/main.go` 用 `ListenAndServe`，不带 TLS）。以下内容都会明文过网：

- 管理员用户名/密码、access/refresh token（`POST /auth/login`）；
- OneBot 的 `access_token`（反向 WS 的查询参数）与全部指令流量。

因此：

- **不要把 8080 直接暴露到公网**；远程访问请在前面放终止 TLS 的反向代理（Nginx / Caddy / Traefik），
  对外提供 HTTPS 与 `wss://`，只允许代理回源到本机 8080。
- 仅在本机或受信内网执行本文的 `http://` / `ws://` 命令。
- 反向代理日志请对 `access_token` 查询参数脱敏（OneBot 令牌等价于连接凭据）。
- 初始密码是明文写在 `admin-credentials.txt` 里的随机串，登录后**立即改密**并删除该文件（见 §6）。
- 登录接口有 IP 限频（5 次/10 分钟），返回 429 时不要循环重试。

## 6. 首登后必做

1. **立即改密**：

   ```bash
   TOKEN=<② 拿到的 access token>
   curl -s -X POST "$BASE/auth/restorePassword" \
     -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
     -d '{"oldPassword":"<初始密码>","newPassword":"<新密码>","confirmPassword":"<新密码>"}'
   # 期望：{"code":200,"msg":"密码修改成功","data":null}
   ```

2. 删除或妥善保管 `admin-credentials.txt` 副本（它只含初始密码，不含新密码）。
3. 登录 WebUI（`$BASE/`）确认页面正常。

## 7. 常见故障速查

| 现象 | 原因 | 处置 |
|------|------|------|
| 容器/进程立刻退出，日志 `exec: no such file or directory` | 在 musl 环境（Alpine）运行了 glibc 动态链接的 Linux 产物 | 换 `debian:12-slim` 等 glibc 镜像；旧版官方镜像（Alpine 基础）请升级或自行构建 |
| 启动日志 `failed to initialize default admin user` / panic | 可执行文件目录不可写，写不了 `admin-credentials.txt` | 去掉只读挂载，或改目录权限 |
| 改配置报 `rename ... device or resource busy` | Docker 下单独挂载了 `config.yaml` 文件 | 改为目录挂载 + `-w`（见 [docker.md](deploy/docker.md)） |
| 页面 404 或空白 | 用的产物没内嵌前端（自建产物未跑 `pnpm build`） | 用官方 Release/镜像，或按 README「托管前端」重新编译 |
| `config.yaml` 一直不生成内容 | 用了**空**文件占位 | 见硬约束 #3 |
| 健康检查通、`/ws/xxx` 返回 200+HTML | 反向 WS 路径与配置不一致 | 核对 `bot.ws_server_path`（默认 `/ws/shiro`）并重启 |
| 日志 `OCR 准备失败` | 模型下载失败（无网络/缺 ca-certificates）或磁盘不足 | 服务本身可用；补网络或手动放置模型到 `data/ocr_models` |
| 登录返回 429 | IP 登录限频（5 次/10 分钟） | 等待或换 IP，不要循环重试 |
| 重启后 token 全部失效 | `config.yaml` 不可写或缺失，JWT 密钥每次随机 | 确保 `config.yaml` 持久化且非空 |
| 取不到初始密码 | 库中已有管理员，`admin-credentials.txt` 只在首启生成 | 用首启时保存的副本；确实丢失只能删库重来（会清空数据） |

## 8. 子文档

- [Docker 部署](deploy/docker.md)
- [Linux 二进制部署（systemd）](deploy/linux.md)
- [Windows 部署](deploy/windows.md)

## 9. 相关文档

- 仓库 [README](../README.md)：项目能力、接口形态、发布流程
- [AGENTS.md](../AGENTS.md)：协作约束与易踩坑点（仓库内保留，不入库）
- [plans/java-to-go-migration-plan.md](../plans/java-to-go-migration-plan.md)：迁移背景与偏差（仓库内保留，不入库）
