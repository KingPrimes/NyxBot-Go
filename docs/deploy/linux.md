# Linux 二进制部署（含 systemd 托管）

适用：**glibc 发行版**（Debian / Ubuntu / CentOS / Rocky / Fedora / Arch 等）。
产物是动态链接 ELF（`libdl.so.2` / `libc.so.6`），**Alpine 等 musl 系统不可用**（实测无法执行，装 `libc6-compat` 也无效）——这类系统请改用 [Docker 部署](docker.md)。macOS 见文末。

## 1. 下载与校验

```bash
ARCH=$(uname -m); case "$ARCH" in x86_64) A=amd64;; aarch64|arm64) A=arm64;; *) echo "不支持的架构: $ARCH"; exit 1;; esac

BASE_URL=https://github.com/KingPrimes/NyxBot-Go/releases/latest/download
curl -fL -o NyxBot            "$BASE_URL/NyxBot-linux-$A"
curl -fL -o SHA256SUMS.txt    "$BASE_URL/SHA256SUMS.txt"
sha256sum -c --ignore-missing SHA256SUMS.txt      # 期望：NyxBot-linux-<arch>: OK
chmod +x NyxBot
```

> Release 为草稿制：`latest/download` 404 时是草稿未发布，请让用户先发布，不要反复重试。

## 2. 目录规划

程序会在**工作目录**生成 `config.yaml` 与 `data/`，在**可执行文件同级目录**写 `admin-credentials.txt`。
推荐 `/opt/nyxbot` 同时满足两者：

```bash
sudo useradd --system --home /opt/nyxbot --shell /usr/sbin/nologin nyxbot 2>/dev/null || true
sudo mkdir -p /opt/nyxbot
sudo mv NyxBot /opt/nyxbot/
sudo chown -R nyxbot:nyxbot /opt/nyxbot     # 首次启动必须可写（写 admin-credentials.txt）
sudo chmod 750 /opt/nyxbot
```

## 3. 首次启动（生成配置与初始管理员）

```bash
sudo -u nyxbot bash -c 'cd /opt/nyxbot && ./NyxBot'
# 期望日志：config.yaml not found, default config created
#           default admin user created, credentials saved to admin-credentials.txt
# Ctrl+C 停止后：
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
curl -s $BASE/api/health
# {"code":200,"msg":"success","data":{"status":"ok"}}

curl -s -i $BASE/ws/shiro | head -1
# HTTP/1.1 400 Bad Request ← 反向 WS 路由已注册

curl -s -X POST $BASE/auth/login -H "Content-Type: application/json" \
  -d '{"userName":"<用户名>","password":"<密码>"}'
# {"code":200,...,"data":{"token":"eyJ...","refreshToken":"eyJ..."}}
```

登录后请立刻改密（`POST /auth/restorePassword`，见 [手册 §5](../ai-deploy.md#5-首登后必做)）。

防火墙放行（按发行版二选一）：

```bash
sudo ufw allow 8080/tcp                                  # Debian/Ubuntu
sudo firewall-cmd --permanent --add-port=8080/tcp && sudo firewall-cmd --reload   # RHEL 系
```

## 6. 修改 OneBot 配置

```bash
# 方式 A：接口（增量合并并写回 config.yaml）
TOKEN=...   # 登录拿到的 access token
curl -s -X POST http://localhost:8080/config/loading \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"isServerOrClient":true,"wsServerUrl":"/ws/shiro","token":"<OneBot 令牌>"}'
# 方式 B：直接编辑 /opt/nyxbot/config.yaml 的 bot 块
sudo systemctl restart nyxbot      # 方式 A 改端口/mode/WS/令牌后同样要重启
```

让 OneBot 客户端以反向 WS 连接 `ws://<主机>:8080/ws/shiro`（`access_token` 作为查询参数），
日志出现 `OneBot connected: <selfID>` 即为接入成功；此前 `GET /config/bot/bots` 会返回
`{"code":500,"msg":"请链接机器人后操作！..."}` 属正常。

## 7. 升级与回滚

```bash
sudo systemctl stop nyxbot
sudo cp -a /opt/nyxbot /opt/nyxbot.bak-$(date +%F)     # 备份（含 config.yaml 与 data/）
sudo -u nyxbot bash -c 'cd /opt/nyxbot && cp NyxBot NyxBot.old && curl -fL -o NyxBot <新版本下载地址> && chmod +x NyxBot'
sudo systemctl start nyxbot
# 回滚：停服 → mv NyxBot.old NyxBot → 启动；数据库结构随版本自动迁移，降级不保证兼容
```

## 8. macOS

流程与 Linux 相同，产物换成 `NyxBot-darwin-arm64`（Apple Silicon）或 `NyxBot-darwin-amd64`（Intel）：

```bash
curl -fL -o NyxBot "<release-url>/NyxBot-darwin-arm64" && chmod +x NyxBot && ./NyxBot
```

首次运行若被 Gatekeeper 拦截，需在「系统设置 → 隐私与安全性」中允许，或 `xattr -d com.apple.quarantine ./NyxBot`。
开机自启可用 `launchd`（`~/Library/LaunchAgents` 下写 plist），不再展开。
