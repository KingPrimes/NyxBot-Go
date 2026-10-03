// Package resources 以内嵌（go:embed）方式携带前端构建产物，使可执行文件自带
// 完整 Web 界面：不依赖运行目录，单独拷到任何位置都能直接提供前端。
//
// 前端源码在独立仓库 https://github.com/KingPrimes/NyxBot-WebUI，产物不随本仓库
// 入库（resources/static 被 .gitignore 排除）：本地构建前需把 WebUI 的
// resources/static 与 resources/templates 放到本目录，CI 由 release-build.yml 的
// frontend job 拉取该仓库执行 pnpm build 后下发。
package resources

import "embed"

// Static 内嵌的前端资源，根目录布局与 NyxBot-WebUI 的构建输出一致：
// static/ 是 Vite 的输出目录（outDir=resources/static，assetsDir=static），
// templates/ 存放被 move-index-html 插件移动后的 index.html。
//
// 模式使用 all: 前缀，是为了让只有 .gitkeep 的干净检出（resources/static/*
// 已被 .gitignore 排除）也能通过编译。因此 resources/static/.gitkeep 必须保留，
// 删掉会导致本包编译失败。
//
//go:embed all:static all:templates
var Static embed.FS
