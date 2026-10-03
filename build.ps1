# 本地 Windows 打包脚本：把本仓库 resources 目录下的前端构建产物内嵌进 NyxBot.exe。
# 前端源码在独立仓库 https://github.com/KingPrimes/NyxBot-WebUI，产物不入库，
# 因此打包前必须先把 WebUI 的 pnpm build 结果放到 resources/ 下（见 README「托管前端」）。
$ErrorActionPreference = "Stop"

$repoRoot = $PSScriptRoot
Set-Location -LiteralPath $repoRoot

$metadata = Get-Content -LiteralPath "build/metadata/app.json" -Raw | ConvertFrom-Json

# ---- 前端产物检查：go:embed 在编译期读取 resources/，缺失则打出的 exe 没有界面 ----
$indexPath = @("resources/templates/index.html", "resources/static/index.html") |
  Where-Object { Test-Path -LiteralPath $_ } |
  Select-Object -First 1
if (-not $indexPath) {
  throw "missing frontend index.html: build NyxBot-WebUI first, then copy its resources/static and resources/templates into $repoRoot\resources"
}

$assetsDir = "resources/static/static"
$assetFiles = @(Get-ChildItem -LiteralPath $assetsDir -Recurse -File -ErrorAction SilentlyContinue)
if ($assetFiles.Count -eq 0) {
  throw "missing or empty frontend assets dir: $assetsDir (expected the pnpm build output resources/static/static/**)"
}

$assetSizeMB = [math]::Round((($assetFiles | Measure-Object -Property Length -Sum).Sum / 1MB), 2)
Write-Host "Frontend to embed: $indexPath + $($assetFiles.Count) asset files ($assetSizeMB MB)"

# ---- 版本资源 + 编译（go:embed 自动把 resources 打进 exe） ----
if (-not (Get-Command goversioninfo -ErrorAction SilentlyContinue)) {
  throw "goversioninfo not found, install it with: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"
}
goversioninfo -icon="build/icons/windows.ico" -o="cmd/server/resource.syso" "build/windows/versioninfo.json"

$ldflags = "-X nyxbot-go/internal/version.Version=$($metadata.version) -X nyxbot-go/internal/version.Commit=local"
$exeName = "$($metadata.executableName).exe"
go build -ldflags $ldflags -o $exeName ./cmd/server

$exeSizeMB = [math]::Round(((Get-Item -LiteralPath $exeName).Length / 1MB), 2)
Write-Host "Built $exeName ($exeSizeMB MB, frontend embedded) with Windows icon and version info."
