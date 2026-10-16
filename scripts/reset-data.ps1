# Wipes the database and loads the clean synthetic starting data and sample documents.
# Stop the API first, and start it again afterwards.
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..\backend")
if (Test-Path "uploads") { Remove-Item -Recurse -Force "uploads" }
go run .\cmd\seed -reset
