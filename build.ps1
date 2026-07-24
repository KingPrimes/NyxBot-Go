$ErrorActionPreference = "Stop"

$metadata = Get-Content -LiteralPath "build/metadata/app.json" -Raw | ConvertFrom-Json

goversioninfo -icon="build/icons/windows.ico" -o="cmd/server/resource.syso" "build/windows/versioninfo.json"

$ldflags = "-X nyxbot-go/internal/version.Version=$($metadata.version) -X nyxbot-go/internal/version.Commit=local"
go build -ldflags $ldflags -o "$($metadata.executableName).exe" ./cmd/server

"Built $($metadata.executableName).exe with Windows icon and version info."
