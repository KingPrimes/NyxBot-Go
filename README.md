# NyxBot-Go

NyxBot-Go 是面向 `NyxBot-WebUI`（独立仓库：<https://github.com/KingPrimes/NyxBot-WebUI>）的 Gin 后端。

## 前端适配

已根据 `NyxBot-WebUI` 的配置做了以下约定：

- 前端开发服务端口：`9527`
- 后端默认服务端口：`8080`
- 前端生产构建输出：`resources/static`（`index.html` 由 WebUI 的 `move-index-html` 插件移到 `resources/templates`）
- 后端接口响应成功码：`code = 200`
- 前端产物在**编译期**由 `go:embed` 打进可执行文件，运行期不读磁盘（见下）

## 目录结构

```text
cmd/server/main.go        # 程序入口
internal/config/config.go # 基础配置
internal/server/router.go # Gin 路由注册
internal/web/static.go    # 内嵌 Vue 产物的托管和 SPA 兜底
resources/assets.go       # go:embed 内嵌前端产物（打包进可执行文件）
resources/static/         # Vue 构建产物（不入库，编译前放入）
resources/templates/      # Vue 构建产物的 index.html
```

## 启动后端

```bash
go mod tidy
go run ./cmd/server
```

健康检查：

```text
GET http://localhost:8080/api/health
```

示例登录：

```text
POST http://localhost:8080/api/auth/login
```

## 托管前端（编译期内嵌）

前端源码不在本仓库，构建产物也不入库（`resources/static` 被 `.gitignore` 排除）。
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

Windows 本地打包用 `pwsh build.ps1`（校验前端产物 → 注入图标与版本信息 → 编译）。

内嵌后的行为：

- 可执行文件自带完整前端，单独拷到任意目录、以任意工作目录启动都能打开页面，
  运行目录**不需要**再放 `resources`。
- 前端更新必须**重新编译**（内嵌内容取自编译时的 `resources/`）。
- CI（`.github/workflows/release-build.yml`）的 `frontend` job 会自动 clone WebUI 仓库并
  `pnpm build`，把产物作为 artifact 下发给各平台构建 job，再编译进二进制。

然后访问：

```text
http://localhost:8080
```

## 环境变量

```text
APP_PORT=8080
GIN_MODE=debug
```
