# Windows 部署

适用：Windows 10/11（amd64）。产物 `NyxBot.exe` 为自包含单文件（前端、字体、SQLite、onnxruntime 均已内嵌），
**无需安装 Go、Node、运行库或 SQLite**。

## 1. 下载与校验

```powershell
$Base = "https://github.com/KingPrimes/NyxBot-Go/releases/latest/download"
New-Item -ItemType Directory -Force -Path C:\NyxBot | Out-Null
Invoke-WebRequest "$Base/NyxBot.exe"      -OutFile C:\NyxBot\NyxBot.exe
Invoke-WebRequest "$Base/SHA256SUMS.txt"  -OutFile C:\NyxBot\SHA256SUMS.txt

(Get-FileHash C:\NyxBot\NyxBot.exe -Algorithm SHA256).Hash.ToLower()
Select-String -Path C:\NyxBot\SHA256SUMS.txt -Pattern "NyxBot.exe"   # 两处哈希应一致
```

> Release 为草稿制：404 说明草稿尚未发布，请让用户先发布。

## 2. 首次运行（生成配置与初始管理员）

在 `C:\NyxBot` 目录下运行（**工作目录决定 `config.yaml` 与 `data\` 的位置**，
`admin-credentials.txt` 固定写在 exe 同级）：

```powershell
cd C:\NyxBot
.\NyxBot.exe
```

- 首次运行 Windows SmartScreen 可能提示「Windows 已保护你的电脑」——产物未做代码签名，选择「更多信息 → 仍要运行」。
- 防火墙弹窗请选择**允许**（或在下一步手动放行 8080）。
- 日志出现 `default config created` 与 `default admin user created, credentials saved to admin-credentials.txt` 即成功。

另开一个 PowerShell 窗口进行验收：

```powershell
$BASE = "http://localhost:8080"
Invoke-RestMethod "$BASE/api/health"
# code=200  msg=success  data.status=ok

Get-Content C:\NyxBot\admin-credentials.txt        # 初始用户名/密码，登录后立即改密
```

改密（先登录拿 token）：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "$BASE/auth/login" -ContentType "application/json" `
  -Body (@{ userName = "<用户名>"; password = "<密码>" } | ConvertTo-Json)
$token = $login.data.token

Invoke-RestMethod -Method Post -Uri "$BASE/auth/restorePassword" `
  -Headers @{ Authorization = "Bearer $token" } -ContentType "application/json" `
  -Body (@{ oldPassword = "<初始密码>"; newPassword = "<新密码>"; confirmPassword = "<新密码>" } | ConvertTo-Json)
# code=200  msg=密码修改成功
```

反向 WS 路由自检（返回 400 表示路由已注册；带令牌时未认证 401）。
这里用系统自带的 `curl.exe`（Win10 1803+ 内置），它同时兼容 Windows PowerShell 5.1 与 PowerShell 7：

```powershell
curl.exe -s -i "$BASE/ws/shiro" | Select-Object -First 1
# HTTP/1.1 400 Bad Request
```

## 3. 放行端口（可选，仅当需要被其它主机访问）

```powershell
# 管理员 PowerShell
New-NetFirewallRule -DisplayName "NyxBot 8080" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow
```

## 4. 配置 OneBot

```powershell
$token = "<管理员 access token>"
Invoke-RestMethod -Method Post -Uri "$BASE/config/loading" `
  -Headers @{ Authorization = "Bearer $token" } -ContentType "application/json" `
  -Body (@{ isServerOrClient = $true; wsServerUrl = "/ws/shiro"; token = "<OneBot 访问令牌>" } | ConvertTo-Json)
# 也可直接编辑 C:\NyxBot\config.yaml 的 bot 块
# 改完端口 / 模式 / WS 路径 / 令牌后需重启进程（关掉窗口重新运行，或重启计划任务）
```

让 OneBot 客户端（NapCat / LLOneBot 等）以反向 WS 连接 `ws://<主机>:8080/ws/shiro`
（配置了令牌则附加 `?access_token=<令牌>`），日志出现 `OneBot connected: <selfID>` 即接入成功。

## 5. 开机自启（任务计划程序）

先写一个启动脚本，避免任务计划程序的默认工作目录变成 `C:\Windows\System32`（那会把
`config.yaml` 与 `data\` 生成到系统目录）：

`C:\NyxBot\start-nyxbot.cmd`

```bat
@echo off
cd /d C:\NyxBot
NyxBot.exe
```

注册任务（管理员 PowerShell / CMD）：

```powershell
schtasks /Create /TN "NyxBot" /TR "C:\NyxBot\start-nyxbot.cmd" /SC ONSTART /RU SYSTEM /RL HIGHEST /F
schtasks /Run /TN "NyxBot"          # 立即启动
schtasks /Query /TN "NyxBot" /V /FO LIST | Select-String "状态|Status"
```

> 以 SYSTEM 运行时，`config.yaml`、`data\`、`admin-credentials.txt` 均属于 SYSTEM 账户；
> 若想让当前用户管理这些文件，把 `/RU SYSTEM` 换成 `/RU <用户名>` 并在图形界面里勾选「不管用户是否登录都要运行」。

## 6. 升级与回滚

```powershell
schtasks /End /TN "NyxBot" 2>$null            # 或直接结束 NyxBot.exe 进程
Stop-Process -Name NyxBot -ErrorAction SilentlyContinue
Copy-Item C:\NyxBot\NyxBot.exe C:\NyxBot\NyxBot.old.exe -Force
Invoke-WebRequest "$Base/NyxBot.exe" -OutFile C:\NyxBot\NyxBot.exe
schtasks /Run /TN "NyxBot"
# 回滚：停进程 → Copy-Item NyxBot.old.exe NyxBot.exe -Force → 重新启动
```

`config.yaml` 与 `data\` 不含在升级替换范围内，保持原样即可；升级前建议整体复制一份 `C:\NyxBot` 备份
（数据库结构随版本自动迁移，降级不保证兼容）。

## 7. 常见问题

| 现象 | 处置 |
|------|------|
| 双击后窗口一闪而过 | 在 PowerShell 里 `.\NyxBot.exe` 运行以查看报错；常见原因是目录不可写（`admin-credentials.txt` 写失败会 panic）或端口被占用 |
| `Invoke-WebRequest` 报 SSL/TLS 错误 | 旧版 Windows PowerShell 5.1 默认协议过旧：先执行 `[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12`，或改用 `curl.exe -L -o NyxBot.exe <下载地址>` |
| 启动报端口占用 | `netstat -ano | findstr :8080` 找到进程；改 `config.yaml` 的 `server.port` 或设环境变量 `APP_PORT` |
| 页面打不开 | 确认访问的是 `http://localhost:8080`（默认端口），且进程仍在运行 |
| 手机/其它机器访问不到 | 检查防火墙规则与 `config.yaml` 的 `server.port`；不要只监听回环 |
| 日志 `OCR 准备失败` | 模型下载失败（网络受限）；服务不受影响，可手动把 det/rec.onnx 放到 `data\ocr_models\` |
