# AWS Lambda CLI Deploy Script
# Encoded in ASCII/UTF-8 (English comments to avoid encoding issues)

param(
    [string]$FunctionName = "",
    [string]$Region = "",
    [string]$DbUrl = "",
    [string]$DbToken = "",
    [string]$ApiKey = "",
    [string]$RoleArn = "",
    [string]$ConfigFile = ".\.env.json"
)

$ErrorActionPreference = "Stop"

Write-Host "=== GNB MemoryMCP Lambda Deploy ===" -ForegroundColor Cyan

# Load config from file if exists (.env.json or .env)
if (Test-Path $ConfigFile) {
    Write-Host "Loading configuration from file: $ConfigFile" -ForegroundColor Gray
    try {
        $fileContent = Get-Content $ConfigFile -Raw | ConvertFrom-Json
        if (-not $FunctionName -and $fileContent.FUNCTION_NAME) { $FunctionName = $fileContent.FUNCTION_NAME }
        if (-not $Region -and $fileContent.REGION) { $Region = $fileContent.REGION }
        if (-not $DbUrl -and $fileContent.DB_URL) { $DbUrl = $fileContent.DB_URL }
        if (-not $DbToken -and $fileContent.DB_TOKEN) { $DbToken = $fileContent.DB_TOKEN }
        if (-not $ApiKey -and $fileContent.API_KEY) { $ApiKey = $fileContent.API_KEY }
        if (-not $RoleArn -and $fileContent.LAMBDA_ROLE_ARN) { $RoleArn = $fileContent.LAMBDA_ROLE_ARN }
    } catch {
        Write-Host "Warning: Failed to parse $ConfigFile as JSON. Checking line-based .env format..." -ForegroundColor Yellow
    }
} elseif (Test-Path ".\.env") {
    Write-Host "Loading configuration from .env file..." -ForegroundColor Gray
    Get-Content ".\.env" | ForEach-Object {
        $line = $_.Trim()
        if ($line -and -not $line.StartsWith("#") -and $line.Contains("=")) {
            $parts = $line.Split("=", 2)
            $k = $parts[0].Trim()
            $v = $parts[1].Trim()
            if (-not $FunctionName -and $k -eq "FUNCTION_NAME") { $FunctionName = $v }
            if (-not $Region -and $k -eq "REGION") { $Region = $v }
            if (-not $DbUrl -and $k -eq "DB_URL") { $DbUrl = $v }
            if (-not $DbToken -and $k -eq "DB_TOKEN") { $DbToken = $v }
            if (-not $ApiKey -and $k -eq "API_KEY") { $ApiKey = $v }
            if (-not $RoleArn -and $k -eq "LAMBDA_ROLE_ARN") { $RoleArn = $v }
        }
    }
}

# Fallback to Environment Variables or Defaults
if (-not $FunctionName) { $FunctionName = if ($env:FUNCTION_NAME) { $env:FUNCTION_NAME } else { "gnb-memorymcp" } }
if (-not $Region) { $Region = if ($env:REGION) { $env:REGION } else { "ap-northeast-3" } }
if (-not $DbUrl) { $DbUrl = $env:DB_URL }
if (-not $DbToken) { $DbToken = $env:DB_TOKEN }
if (-not $ApiKey) { $ApiKey = $env:API_KEY }
if (-not $RoleArn) { $RoleArn = $env:LAMBDA_ROLE_ARN }

Write-Host "Target Function: $FunctionName" -ForegroundColor Cyan
Write-Host "Target Region  : $Region" -ForegroundColor Cyan

# Helper function to create temporary environment JSON file for AWS CLI (UTF-8 No BOM)
function New-TempEnvJsonFile {
    param([string]$url, [string]$token, [string]$key)
    $tempPath = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), "lambda-env-$([guid]::NewGuid().ToString()).json")
    $envHash = @{
        Variables = @{
            DB_URL   = [string]$url
            DB_TOKEN = [string]$token
            API_KEY  = [string]$key
        }
    }
    $jsonString = $envHash | ConvertTo-Json -Compress
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($tempPath, $jsonString, $utf8NoBom)
    return $tempPath
}

# 1. Check AWS CLI availability
if (-not (Get-Command aws -ErrorAction SilentlyContinue)) {
    Write-Error "Error: 'aws' CLI command not found. Please install AWS CLI v2 and add it to PATH."
    exit 1
}

# 2. Check lambda.zip existence
$zipPath = ".\bin\lambda.zip"
if (-not (Test-Path $zipPath)) {
    Write-Host "lambda.zip not found. Building binary package..." -ForegroundColor Yellow
    powershell -File .\build-lambda.ps1
    if (-not (Test-Path $zipPath)) {
        Write-Error "Error: Failed to build lambda.zip."
        exit 1
    }
}

# 3. Check existing Lambda function
Write-Host "Checking AWS Lambda function '$FunctionName' in region '$Region'..." -ForegroundColor Cyan
$functionExists = $true
$checkOutput = aws lambda get-function --function-name $FunctionName --region $Region 2>&1
if ($LASTEXITCODE -ne 0) {
    $functionExists = $false
}

if ($functionExists) {
    # 4. Update existing function
    Write-Host "Updating code for existing Lambda function '$FunctionName'..." -ForegroundColor Green
    aws lambda update-function-code `
        --function-name $FunctionName `
        --zip-file "fileb://$zipPath" `
        --region $Region | Out-Null

    if ($LASTEXITCODE -ne 0) {
        Write-Error "Error: Failed to update Lambda function code."
        exit 1
    }

    # Update environment variables if provided
    if ($DbUrl -and $ApiKey) {
        Write-Host "Waiting for Lambda function update to complete..." -ForegroundColor Gray
        aws lambda wait function-updated --function-name $FunctionName --region $Region

        Write-Host "Updating environment variables..." -ForegroundColor Cyan
        $tempEnvFile = New-TempEnvJsonFile -url $DbUrl -token $DbToken -key $ApiKey
        try {
            aws lambda update-function-configuration `
                --function-name $FunctionName `
                --environment "file://$tempEnvFile" `
                --region $Region | Out-Null
        } finally {
            if (Test-Path $tempEnvFile) { Remove-Item $tempEnvFile -Force }
        }
    }
} else {
    # 5. Create new function
    Write-Host "Lambda function '$FunctionName' does not exist. Creating new function..." -ForegroundColor Yellow

    if (-not $RoleArn) {
        Write-Error "Error: IAM Role ARN is required for creating a new function. Set LAMBDA_ROLE_ARN in .env.json, environment variables, or -RoleArn."
        exit 1
    }

    if (-not $DbUrl -or -not $ApiKey) {
        Write-Error "Error: DB_URL and API_KEY are required for creation."
        exit 1
    }

    $tempEnvFile = New-TempEnvJsonFile -url $DbUrl -token $DbToken -key $ApiKey
    try {
        aws lambda create-function `
            --function-name $FunctionName `
            --runtime provided.al2023 `
            --role $RoleArn `
            --handler bootstrap `
            --architectures x86_64 `
            --zip-file "fileb://$zipPath" `
            --environment "file://$tempEnvFile" `
            --region $Region | Out-Null
    } finally {
        if (Test-Path $tempEnvFile) { Remove-Item $tempEnvFile -Force }
    }

    if ($LASTEXITCODE -ne 0) {
        Write-Error "Error: Failed to create Lambda function."
        exit 1
    }

    # Enable Function URL
    Write-Host "Enabling Function URL..." -ForegroundColor Cyan
    aws lambda create-function-url-config `
        --function-name $FunctionName `
        --auth-type NONE `
        --region $Region | Out-Null

    # Add public invocation permission
    aws lambda add-permission `
        --function-name $FunctionName `
        --statement-id FunctionURLAllowPublicAccess `
        --action lambda:InvokeFunctionUrl `
        --principal "*" `
        --function-url-auth-type NONE `
        --region $Region | Out-Null
}

# 6. Retrieve Function URL
try {
    $urlJson = aws lambda get-function-url-config --function-name $FunctionName --region $Region
    $urlConfig = $urlJson | ConvertFrom-Json
    Write-Host "`nDeployment successfully completed!" -ForegroundColor Green
    Write-Host "Function URL: $($urlConfig.FunctionUrl)" -ForegroundColor Yellow
} catch {
    Write-Host "`nDeployment completed! (Could not retrieve Function URL, please check AWS console)" -ForegroundColor Green
}
