# NyxBot-Go

NyxBot-Go 是面向 `NyxBot-WebUI` 的 Gin 后端基础骨架。

## 前端适配

已根据 `D:\Demos\NyxBot-WebUI` 的配置做了以下约定：

- 前端开发服务端口：`9527`
- 后端默认服务端口：`8080`
- 前端生产构建输出：`resources/static`
- 后端接口响应成功码：`code = 200`
- Gin 托管 Vue 构建产物目录：`./resources/static`

## 目录结构

```text
cmd/server/main.go        # 程序入口
internal/config/config.go # 基础配置
internal/server/router.go # Gin 路由注册
internal/web/static.go    # Vue 静态文件托管和 SPA 兜底
resources/static/         # 放置 Vue 构建后的文件
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

## 托管前端

在 `D:\Demos\NyxBot-WebUI` 执行：

```bash
pnpm build
```

该前端项目的 `vite.config.ts` 已配置 `outDir: 'resources/static'`。如果要由当前 Gin 项目托管，需要将构建结果复制到当前项目：

```text
D:\Demos\NyxBot-WebUI\resources\static -> D:\Demos\NyxBot-Go\resources\static
```

然后访问：

```text
http://localhost:8080
```

## 环境变量

```text
APP_HOST=0.0.0.0
APP_PORT=8080
APP_STATIC_DIR=./resources/static
GIN_MODE=debug
```
