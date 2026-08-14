# Baixa o driver wintun.dll (WireGuard LLC, https://www.wintun.net/) e o
# coloca em apps/xvpn-client/internal/platform/windows/wintun/wintun.dll, de onde
# `go:embed` o inclui no binário — ver apps/xvpn-client/internal/platform/windows/wintun.go
# e PLAN.md §11.1. wintun.dll é um binário de terceiros e por isso nunca é
# commitado neste repositório (ver .gitignore).
#
# Uso (PowerShell, no Windows, a partir de apps/xvpn-client/):
#   .\build\windows\fetch-wintun.ps1
#
# IMPORTANTE: confira o SHA256 do arquivo baixado contra o publicado em
# https://www.wintun.net/ antes de usar em produção — este script não fixa
# um hash porque a versão "latest" muda; para builds reproduzíveis, ajuste
# $WintunVersion para uma versão específica e adicione o hash esperado.

param(
    [string]$WintunVersion = "0.14.1"
)

$ErrorActionPreference = "Stop"

$zipUrl = "https://www.wintun.net/builds/wintun-$WintunVersion.zip"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$destDir = Join-Path $scriptDir "..\..\internal\platform\windows\wintun"
$tmpZip = Join-Path $env:TEMP "wintun-$WintunVersion.zip"
$tmpExtract = Join-Path $env:TEMP "wintun-$WintunVersion-extract"

Write-Host "Baixando $zipUrl ..."
Invoke-WebRequest -Uri $zipUrl -OutFile $tmpZip

Write-Host "SHA256 do arquivo baixado (confira em https://www.wintun.net/ antes de confiar):"
Get-FileHash -Algorithm SHA256 $tmpZip | Select-Object Hash

if (Test-Path $tmpExtract) { Remove-Item -Recurse -Force $tmpExtract }
Expand-Archive -Path $tmpZip -DestinationPath $tmpExtract

$dll = Join-Path $tmpExtract "wintun\bin\amd64\wintun.dll"
if (-not (Test-Path $dll)) {
    throw "wintun.dll (amd64) não encontrado no zip extraído em $tmpExtract"
}

New-Item -ItemType Directory -Force -Path $destDir | Out-Null
Copy-Item -Force $dll (Join-Path $destDir "wintun.dll")

Write-Host "wintun.dll copiado para $destDir\wintun.dll"
Write-Host "Pronto para 'wails3 build' / 'task build' no Windows."
