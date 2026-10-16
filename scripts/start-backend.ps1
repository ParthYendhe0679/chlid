# Builds and starts the Child Connect API (http://localhost:8080/api/v1). Settings come from backend/.env.
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..\backend")
go build -o bin\childconnect-api.exe .\cmd\api
.\bin\childconnect-api.exe
