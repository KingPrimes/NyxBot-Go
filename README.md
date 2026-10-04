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
`admin-credentials.txt`（随机管理员账号）。库里已有用户时重启不会改动该文件；数据库被删除/清空后
会重新生成管理员并**覆盖**该文件（旧密码已失效，不覆盖就无法登录）。健康检查：

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
pwsh build.ps1                      # 校验前端产物 → 注入图标与版本信息 → 产出 NyxBot.exe
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

## 托管前端（编译期内嵌）

前端源码不在本仓库（独立仓库 <https://github.com/KingPrimes/NyxBot-WebUI>），构建产物也不入库
（`resources/static` 被 `.gitignore` 排除）。产物在**编译期**由 `go:embed`（`resources/assets.go`）
打进可执行文件，运行期不读磁盘。

打包前先构建前端并把它放进本仓库的 `resources/`。下面的命令**起点是 NyxBot-Go 仓库根目录**，
且假定 `NyxBot-WebUI` 与它同级：

```bash
# 1. 构建前端（产物落在 ../NyxBot-WebUI/resources；已经 clone 过就跳过 git clone）
cd ..
git clone https://github.com/KingPrimes/NyxBot-WebUI.git
cd NyxBot-WebUI
pnpm install
pnpm build

# 2. 把产物拷进后端仓库的 resources/（static 与 templates 都要，SPA 入口 index.html 在 templates 里）
#    源写成 "目录/." 表示拷贝目录内容，这样目标目录已存在时不会被多套一层
cp -r resources/static/.    ../NyxBot-Go/resources/static/
cp -r resources/templates/. ../NyxBot-Go/resources/templates/

# 3. 回到后端仓库再编译：go:embed 会把 resources/ 打进可执行文件
cd ../NyxBot-Go
go build -o NyxBot ./cmd/server
```

内嵌后的行为：

- 可执行文件自带完整前端，单独拷到任意目录、以任意工作目录启动都能打开页面，
  运行目录**不需要**再放 `resources`。
- 前端更新必须**重新编译**（内嵌内容取自编译时的 `resources/`）。
- CI（`.github/workflows/release-build.yml`）的 `frontend` job 会自动 clone WebUI 仓库并
  `pnpm build`，把产物作为 artifact 下发给各平台构建 job，再编译进二进制。

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
  web/                        # 内嵌 Vue 产物托管 + SPA 兜底（资源全部来自 go:embed，运行期不读磁盘）
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
resources/assets.go           # go:embed 内嵌 resources/static + resources/templates（前端随二进制分发）
resources/static/             # 前端构建产物（不入库，保留 .gitkeep；SPA 入口 index.html 在 resources/templates/）
```

## 运行期生成物

以下路径**不应提交**（已在 `.gitignore` 中）：

- `config.yaml`：运行期配置，缺失时自动生成。
- `admin-credentials.txt`：随机初始管理员账号（库中已有用户时不动它；数据库被删除后会覆盖为新的账号密码）。
- `data/`：SQLite 库、`data/logs/`、WorldState 快照 `data/status`、仲裁缓存 `data/arbitration`、未翻译清单 `data/UntranslatedRelicsRewardsName.json`（遗物导入与世界状态翻译未命中时按 uniqueName 去重登记，供人工补齐后回填 `state_translation`）。
- `temp/`：绘图测试输出的 PNG。
- `resources/static/`、`resources/templates/`：前端构建产物（编译期由 `go:embed` 打进二进制）。
- `cmd/server/resource.syso`：`build.ps1` / CI 生成。

## 发布

推送 `v*` tag（或手动触发 `workflow_dispatch`）运行 `.github/workflows/release-build.yml`，
构建 linux/macOS/Windows 三平台二进制（版本号取自 tag，缺省回退 `build/metadata/app.json`），
并推送多架构 Docker 镜像到 `kingprimes/nyxbot-go`（Docker Hub）与 `ghcr.io/<owner>/nyxbot-go`。
该工作流先由 `frontend` job 拉取 `KingPrimes/NyxBot-WebUI` 执行 `pnpm build`，把产物下发给各平台后再编译
（前端因此内嵌在二进制里）；Docker 镜像直接复用 CI 构建的 linux 产物，不在镜像内重复编译，也无需再拷贝 `resources`。

前端版本**默认取该仓库默认分支的最新提交**。需要固定（例如前端改了不兼容的接口、或想让某个发布可复现）时二选一：

- 手动触发时填 `webui_ref`：分支 / 标签 / 提交 SHA；
- 在仓库 `Settings → Secrets and variables → Actions → Variables` 建 `WEBUI_REF`：tag 触发的发布同样生效，适合长期钉住某个前端版本。

优先级为 `webui_ref` > `WEBUI_REF` > 默认分支；构建日志会打印实际用到的前端 commit
（`frontend commit : <sha>`），便于回溯某个二进制里内嵌的是哪版前端。
