# AWS Lambda (provided.al2023) Go Build Script
# Encoded in ASCII/UTF-8 (English messages to prevent PowerShell encoding issues)

$ErrorActionPreference = "Stop"

# Create output directory
$binDir = ".\bin"
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null
}

$bootstrapPath = ".\bootstrap"

# Clean up existing temp files
if (Test-Path $bootstrapPath) {
    Remove-Item $bootstrapPath -Force
}
if (Test-Path "$bootstrapPath.exe") {
    Remove-Item "$bootstrapPath.exe" -Force
}

Write-Host "Building binary for AWS Lambda (GOOS=linux, GOARCH=amd64, CGO_ENABLED=0)..." -ForegroundColor Cyan

# Run go build inside cmd.exe to ensure env vars are applied
$buildCommand = 'set GOOS=linux&& set GOARCH=amd64&& set CGO_ENABLED=0&& go build -ldflags="-s -w" -o ' + $bootstrapPath + ' ./cmd/lambda/main.go'
cmd.exe /c $buildCommand

if ($LastExitCode -ne 0) {
    Write-Error "Error: go build failed with exit code $LastExitCode."
}

# Strict check for the generated binary
$item = Get-Item $bootstrapPath -ErrorAction SilentlyContinue
if ($null -eq $item -or $item.PSIsContainer) {
    if (Test-Path "$bootstrapPath.exe") {
        Write-Error "Error: GOOS=linux did not apply. bootstrap.exe was created instead."
    } else {
        Write-Error "Error: Build artifact not found."
    }
}

Write-Host "Build successful. Creating zip archive..." -ForegroundColor Cyan

# Remove existing zip if any
$zipPath = "$binDir\lambda.zip"
if (Test-Path $zipPath) {
    Remove-Item $zipPath -Force
}

# Compress to zip
Compress-Archive -Path $bootstrapPath -DestinationPath $zipPath -Force

# Clean up temp binary
Remove-Item $bootstrapPath -Force

Write-Host "AWS Lambda package created: $zipPath" -ForegroundColor Green
