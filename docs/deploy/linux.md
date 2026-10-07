# Linux 二进制部署（含 systemd 托管）

适用：**glibc 发行版**（Debian / Ubuntu / CentOS / Rocky / Fedora / Arch 等）。
产物是动态链接 ELF（`libdl.so.2` / `libc.so.6`），**Alpine 等 musl 系统不可用**（实测无法执行，装 `libc6-compat` 也无效）——这类系统请改用 [Docker 部署](docker.md)。macOS 见文末。

> 服务只监听明文 HTTP/WS，**不要把 8080 直接暴露到公网**；远程访问需 TLS 反向代理（HTTPS / `wss://`），
> 详见 [手册的安全说明](../ai-deploy.md#5-安全说明公网部署必读)。

## 1. 下载与校验

**按清单里的文件名下载**——存成别的名字会让 `sha256sum -c` 跳过校验（`--ignore-missing` 只检查存在的文件），
看起来通过、实际没验：

```bash
case "$(uname -m)" in
  x86_64)          A=amd64 ;;
  aarch64|arm64)   A=arm64 ;;
  *) echo "不支持的架构: $(uname -m)"; exit 1 ;;
esac

BASE_URL=https://github.com/KingPrimes/NyxBot-Go/releases/latest/download
curl -fL -O "$BASE_URL/NyxBot-linux-$A"          # 文件名必须与清单一致
curl -fL -o SHA256SUMS.txt "$BASE_URL/SHA256SUMS.txt"
sha256sum -c --ignore-missing SHA256SUMS.txt     # 期望：NyxBot-linux-<arch>: OK
chmod +x "NyxBot-linux-$A"
./"NyxBot-linux-$A" --version                    # 复核实际拿到的版本，再动部署
```

> Release 是草稿制：`releases/latest/download` 在「新版本还是草稿」时会**返回上一个已发布版本**（不是 404）。
> 要固定版本请把 `BASE_URL` 换成 `…/releases/download/<tag>`。

## 2. 目录规划

程序会在**工作目录**生成 `config.yaml` 与 `data/`，在**可执行文件同级目录**写 `admin-credentials.txt`。
推荐 `/opt/nyxbot` 同时满足两者：

```bash
sudo useradd --system --home /opt/nyxbot --shell /usr/sbin/nologin nyxbot 2>/dev/null || true
sudo mkdir -p /opt/nyxbot
sudo install -m 0755 "NyxBot-linux-$A" /opt/nyxbot/NyxBot
sudo chown -R nyxbot:nyxbot /opt/nyxbot     # 首次启动必须可写（写 admin-credentials.txt）
sudo chmod 750 /opt/nyxbot
```

## 3. 首次启动（生成配置与初始管理员）

```bash
sudo -u nyxbot bash -c 'cd /opt/nyxbot && ./NyxBot'
# 期望日志：config.yaml not found, default config created
#           default admin user created, credentials saved to admin-credentials.txt
# Ctrl+C 停止后立即保存凭据（库中已有管理员后不会再生成）：
sudo install -m 600 -o nyxbot -g nyxbot /opt/nyxbot/admin-credentials.txt /opt/nyxbot/admin-credentials.initial.txt
sudo grep -E '^(username|password):' /opt/nyxbot/admin-credentials.txt   # 初始账号
```

若要把服务开在别的端口，用环境变量覆盖顶层字段即可（`APP_PORT=9000`），或稍后编辑 `config.yaml` 的 `server.port`。

## 4. systemd 托管

`/etc/systemd/system/nyxbot.service`：

```ini
[Unit]
Description=NyxBot Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=nyxbot
Group=nyxbot
WorkingDirectory=/opt/nyxbot
ExecStart=/opt/nyxbot/NyxBot
Restart=on-failure
RestartSec=5
# 需要改端口/日志开关时取消注释（环境变量只覆盖顶层字段）
# Environment=APP_PORT=8080
# Environment=APP_LOG_CONSOLE=false

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now nyxbot
systemctl status nyxbot --no-pager
journalctl -u nyxbot -n 50 --no-pager      # 看启动日志
```

> `WorkingDirectory` 决定 `config.yaml` / `data/` 的落点，`User` 决定这些文件属主；两者都要与 §2 一致。

## 5. 验收

```bash
BASE=http://localhost:8080
USER=<用户名>
PASS=<密码>

curl -s "$BASE/api/health"
# {"code":200,"msg":"success","data":{"status":"ok"}}

curl -s -i "$BASE/ws/shiro" | head -1
# HTTP/1.1 400 Bad Request ← 反向 WS 路由已注册

curl -s -X POST "$BASE/auth/login" -H "Content-Type: application/json" \
  -d "{\"userName\":\"$USER\",\"password\":\"$PASS\"}"
# {"code":200,...,"data":{"token":"eyJ...","refreshToken":"eyJ..."}}
```

登录后请立刻改密（`POST /auth/restorePassword`，见 [手册 §6](../ai-deploy.md#6-首登后必做)）。

防火墙放行（按发行版二选一；**公网场景请改走 TLS 反代**）：

```bash
sudo ufw allow 8080/tcp                                  # Debian/Ubuntu
sudo firewall-cmd --permanent --add-port=8080/tcp && sudo firewall-cmd --reload   # RHEL 系
```

## 6. 修改 OneBot 配置

```bash
TOKEN=<登录拿到的 access token>

# 方式 A：接口（增量合并并写回 config.yaml）
curl -s -X POST "$BASE/config/loading" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"isServerOrClient":true,"wsServerUrl":"/ws/shiro","token":"<OneBot 令牌>"}'
# 方式 B：直接编辑 /opt/nyxbot/config.yaml 的 bot 块
sudo systemctl restart nyxbot      # 方式 A 改端口/mode/WS/令牌后同样要重启
```

让 OneBot 客户端以反向 WS 连接 `ws://<主机>:8080/ws/shiro`（`access_token` 作为查询参数；
跨不可信网络请走 `wss://` + TLS 代理），日志出现 `OneBot connected: <selfID>` 即为接入成功；
此前 `GET /config/bot/bots` 会返回 `{"code":500,"msg":"请链接机器人后操作！..."}` 属正常。

## 7. 升级与回滚

先下载到临时位置、校验通过后再替换运行文件——直接 `curl -o /opt/nyxbot/NyxBot` 会在下载失败时
留下截断的二进制，而服务已经停了：

```bash
BASE_URL=https://github.com/KingPrimes/NyxBot-Go/releases/latest/download
A=amd64                                  # 按 §1 的 uname -m 结果填
TMP=$(mktemp -d)

curl -fL -o "$TMP/NyxBot-linux-$A"    "$BASE_URL/NyxBot-linux-$A"
curl -fL -o "$TMP/SHA256SUMS.txt"     "$BASE_URL/SHA256SUMS.txt"
( cd "$TMP" && sha256sum -c --ignore-missing SHA256SUMS.txt )   # 期望：OK
"$TMP/NyxBot-linux-$A" --version                                 # 复核版本

sudo cp -a /opt/nyxbot /opt/nyxbot.bak-$(date +%F)               # 备份（含 config.yaml 与 data/）
sudo systemctl stop nyxbot
sudo install -m 0755 -o nyxbot -g nyxbot "$TMP/NyxBot-linux-$A" /opt/nyxbot/NyxBot.new
sudo mv /opt/nyxbot/NyxBot /opt/nyxbot/NyxBot.old                # 同目录改名，原子
sudo mv /opt/nyxbot/NyxBot.new /opt/nyxbot/NyxBot
sudo systemctl start nyxbot
rm -rf "$TMP"
```

回滚：`systemctl stop nyxbot` → `mv NyxBot.old NyxBot` → 启动。数据库结构随版本自动迁移，降级不保证兼容，
必要时连备份一起回退（恢复前先停服，避免对运行中的 SQLite 做非一致性拷贝）。

## 8. macOS

流程与 Linux 相同，产物换成 `NyxBot-darwin-arm64`（Apple Silicon）或 `NyxBot-darwin-amd64`（Intel）：

```bash
A=arm64    # Intel 机器改成 amd64
BASE_URL=https://github.com/KingPrimes/NyxBot-Go/releases/latest/download
curl -fL -O "$BASE_URL/NyxBot-darwin-$A"
curl -fL -o SHA256SUMS.txt "$BASE_URL/SHA256SUMS.txt"
sha256sum -c --ignore-missing SHA256SUMS.txt     # 期望：NyxBot-darwin-<arch>: OK
chmod +x "NyxBot-darwin-$A"
xattr -d com.apple.quarantine "NyxBot-darwin-$A" 2>/dev/null || true   # 去掉隔离标记，避免 Gatekeeper 拦截
./"NyxBot-darwin-$A" --version
```

首次运行若仍被 Gatekeeper 拦截，需在「系统设置 → 隐私与安全性」中允许。
开机自启可用 `launchd`（`~/Library/LaunchAgents` 下写 plist），不再展开。
