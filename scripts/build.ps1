$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

$env:CGO_ENABLED = "0"
$ldflags = "-s -w"

Write-Host "==> 构建服务端 (linux/amd64)"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -trimpath -ldflags=$ldflags -o modernfs-linux-amd64 .

Write-Host "==> 构建配置向导 (windows/amd64)"
Push-Location cmd/config-writer
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -trimpath -ldflags=$ldflags -o config-writer.exe .
Pop-Location

Write-Host "==> 完成"
Get-ChildItem -LiteralPath "modernfs-linux-amd64", "cmd/config-writer/config-writer.exe" |
    Select-Object Name, @{n = 'SizeMB'; e = { [math]::Round($_.Length / 1MB, 2) } } |
    Format-Table -AutoSize
