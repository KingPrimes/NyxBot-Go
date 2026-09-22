# NyxBot-Go

NyxBot-Go 是 [NyxBot](https://github.com/KingPrimes/NyxBot)（Java Spring Boot）迁移到 Go/Gin 的后端实现，
目标是在**不修改前端**的前提下完整兼容 `D:\Demos\NyxBot-WebUI`（Vue3 + Soybean Admin）。

当前状态：阶段 1-9 已完成，阶段 10（Warframe Bot 指令）实现 22/40，阶段 11（定时任务）未开始。
完整背景、决策、前端接口映射与逐阶段进度见 [`plans/java-to-go-migration-plan.md`](plans/java-to-go-migration-plan.md)。
协作约定与易踩坑点见 [`AGENTS.md`](AGENTS.md)。

## 技术栈

| 层 | 选型 |
|----|------|
| Web | Gin v1.10 |
| ORM / 数据库 | GORM + `glebarez/sqlite`（pure-Go，无需 CGO） |
| 认证 | JWT 双令牌（access 2h / refresh 7d）+ bcrypt |
| OneBot | ZeroBot v1.8.2（反向 WS 用自定义 Driver 挂到 Gin 主服务） |
| 绘图 | `fogleman/gg` + 嵌入字体（无插件机制） |
| 配置 | `config.yaml`（唯一真相）+ 环境变量覆盖 |
| Go 版本 | 1.25 |

## 快速开始

```bash
go mod tidy
go run ./cmd/server
```

首次启动会自动创建 `config.yaml`（含随机 JWT 密钥）与 `data/nyxbot.db`，并在可执行文件同级目录写出
`admin-credentials.txt`（随机管理员账号，**已存在则跳过**）。健康检查：

```bash
curl http://localhost:8080/api/health      # {"code":200,"msg":"success","data":{"status":"ok"}}
```

常用命令：

```bash
go build ./...                      # 编译全部包
go test  ./...                      # 全量测试（测试全部集中在 tests/ 包）
go test  ./tests -run TestXxx -v    # 单用例
go vet ./...                        # 静态检查
```

> 若在无法读取 `.git` 的容器/沙箱里构建，追加 `-buildvcs=false`。

Windows 带图标与版本信息的本地构建（需先 `go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest`）：

```powershell
pwsh build.ps1                      # 产出 NyxBot.exe
```

## 与前端对接

前端 `NyxBot-WebUI` 通过 Soybean Admin 的 `VITE_SERVICE_BASE_URL` 拼接请求路径，Go 侧必须保持同样的路径形态：

| 场景 | 前端请求 | 到达后端 |
|------|---------|---------|
| 开发（`pnpm dev`，`VITE_HTTP_PROXY=Y`） | `/proxy-default/auth/login` | Vite 代理 rewrite 掉 `/proxy-default` → `http://localhost:8080/auth/login` |
| 生产（`.env.prod` 的 `VITE_SERVICE_BASE_URL=/`） | `/auth/login` | 同源，Gin 直接处理 |

因此后端路由**不带** `/api` 前缀（`/auth/*`、`/config/**`、`/log/**`、`/data/warframe/**`、`/sse/*`）；
仅通用接口在 `/api` 下（`/api/health`、`/api/logs/**`）。

前端开发服务端口 `9527`（preview `9725`），后端默认监听 `8080`。

响应格式恒为 `{ code, msg, data }`，成功 `code=200`，未认证 `401`，分页数据为 `{ total, size, current, records }`。

## 托管前端静态资源

前端构建产物输出到其自身的 `resources/static`，需要拷贝到本项目：

```bash
# 在 D:\Demos\NyxBot-WebUI 下
pnpm build
```

```text
D:\Demos\NyxBot-WebUI\resources\static  ->  D:\Demos\NyxBot-Go\resources\static
```

之后访问 `http://localhost:8080` 即可（SPA 兜底：未命中的路由返回 `index.html`）。

## 配置

`config.yaml` 是唯一真相，**不入版本库**——文件缺失时 `config.Load` 会自动写入带字段注释的默认值，
因此新克隆直接 `go run` 即可。可通过前端配置页（`GET/POST /config/loading`）修改并即时生效；
直接编辑文件则需重启（文件变更监听未实现）。

六大配置块：`server` / `database` / `log` / `bot` / `auth` / `warframe`。
其中 `bot` 块对应 ZeroBot 参数：`mode`（`server` 反向 WS / `client` 正向 WS）、 `ws_server_path`（默认 `/ws/shiro`）、
`ws_client_url`、`access_token`、`wait_n`、`plugin_prefix`（指令是否需 @ 触发）。

环境变量只覆盖下列顶层字段：

```text
APP_PORT=8080               # 监听端口
GIN_MODE=release            # debug / release / test
APP_REQUEST_LOG=true        # 是否记录 HTTP 请求日志
DB_PATH=data/nyxbot.db      # SQLite 文件路径
APP_STARTUP_LOG=false       # 关闭启动初始化日志
APP_LOG_CONSOLE=false       # 关闭控制台日志输出
JWT_SECRET=<hex>            # JWT 签名密钥
```

## 目录结构

```text
cmd/server/main.go            # 入口：logging.Init → config.Load → database.Init → server.NewRouter
internal/
  config/                     # config.yaml 加载/生成 + Runtime 热更新
  version/                    # ldflags 注入的 Version / Commit / ProductName
  logging/                    # 自实现 slog 风格日志 + 文件轮转 + SSE 订阅总线
  response/                   # 统一 {code,msg,data} 封装 + PageData
  server/                     # Gin 路由、CORS/Recovery/Logger、SSE 日志、/api/logs/**
  system/                     # /config/loading 读写、/log/** 查询
  web/                        # Vue dist 托管 + SPA 兜底
  auth/                       # /auth/* 双令牌、黑名单、IP 登录限频
  bot/                        # 在线 Bot 快照目录 + /config/bot/**（管理员、黑白名单）
  onebot/                     # OneBot 连接层 + 指令注册/权限/日志 + 各指令 handler
  database/                   # GORM 初始化、AutoMigrate、首启随机管理员
  model/                      # GORM 模型：system / bot / plugin / warframe
  enum/                       # nyxbot（指令权限映射）、drawplugin（绘图枚举）
  draw/                       # 绘图实现（gg + 嵌入字体）
  warframe/                   # WorldState 轮询、仲裁缓存、市场查询、导入/导出、/data/warframe/**
  warframe/cycle/             # 各平原周期计算
tests/                        # 黑盒测试（被测包内不放 *_test.go）
build/                        # app.json 元数据、图标、平台打包资源
resources/static/             # 前端构建产物（不入库，保留 .gitkeep）
```

## 运行期生成物

以下路径**不应提交**（已在 `.gitignore` 中）：

- `config.yaml`：运行期配置，缺失时自动生成。
- `admin-credentials.txt`：首启生成的随机管理员账号。
- `data/`：SQLite 库、`data/logs/`、WorldState 快照 `data/status`、仲裁缓存 `data/arbitration`。
- `temp/`：绘图测试输出的 PNG。
- `resources/static/`：前端构建产物。
- `cmd/server/resource.syso`：`build.ps1` / CI 生成。

## 发布

推送 `v*` tag（或手动触发 `workflow_dispatch`）运行 `.github/workflows/release-build.yml`，
构建 linux/macOS/Windows 三平台二进制（版本号取自 tag，缺省回退 `build/metadata/app.json`），
并推送多架构 Docker 镜像到 `kingprimes/nyxbot-go`（Docker Hub）与 `ghcr.io/<owner>/nyxbot-go`。
Docker 镜像直接复用 CI 构建的 linux 产物，不在镜像内重复编译。
