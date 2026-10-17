# Builds and starts the website in production mode (http://localhost:3000). Start the API first.
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..\frontend")
if (-not (Test-Path ".env.local")) { "NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1" | Out-File -Encoding utf8 .env.local }
if (-not (Test-Path "node_modules")) { npm ci }
npm run build
npm run start
