# MCP Specifications 2026-07-28 Rigorous Verification Script

$mcpURL = "https://oajuohz4tofchqb4hy62yzq65m0paryw.lambda-url.ap-northeast-3.on.aws"

Write-Host "==========================================" -ForegroundColor Yellow
Write-Host " MCP 2026-07-28 Specification Audit Test  " -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Yellow

# 1. initialize request audit
Write-Host "`n[TEST 1] Unauthenticated MCP initialize Request" -ForegroundColor Cyan
$initReq = @{
    jsonrpc = "2.0"
    method  = "initialize"
    id      = 1
    params  = @{
        protocolVersion = "2026-07-28"
        capabilities    = @{}
        clientInfo      = @{ name = "Gemini Spark"; version = "1.0.0" }
    }
} | ConvertTo-Json -Depth 5

try {
    $resp1 = Invoke-WebRequest -Uri "$mcpURL/" -Method Post -ContentType "application/json" -Body $initReq -UseBasicParsing
    Write-Host "  -> Status Code       : $($resp1.StatusCode) (Expected: 200)" -ForegroundColor Green
    Write-Host "  -> Content-Type      : $($resp1.Headers['Content-Type'])" -ForegroundColor Green
    Write-Host "  -> Mcp-Protocol-Version: $($resp1.Headers['Mcp-Protocol-Version'])" -ForegroundColor Green
    Write-Host "  -> Mcp-Session-Id    : $($resp1.Headers['Mcp-Session-Id'])" -ForegroundColor Green
    Write-Host "  -> Response Body     :" -ForegroundColor Gray
    $resp1.Content
} catch {
    Write-Host "  -> FAILED: $_" -ForegroundColor Red
}

# 2. OAuth Discovery audit
Write-Host "`n[TEST 2] OAuth Authorization Server Discovery Metadata" -ForegroundColor Cyan
try {
    $resp2 = Invoke-WebRequest -Uri "$mcpURL/.well-known/oauth-authorization-server" -Method Get -UseBasicParsing
    Write-Host "  -> Status Code       : $($resp2.StatusCode) (Expected: 200)" -ForegroundColor Green
    Write-Host "  -> Response Body     :" -ForegroundColor Gray
    $resp2.Content
} catch {
    Write-Host "  -> FAILED: $_" -ForegroundColor Red
}

# 3. GET / & HEAD / audit
Write-Host "`n[TEST 3] GET / and HEAD / Liveness Probe" -ForegroundColor Cyan
try {
    $rGet = Invoke-WebRequest -Uri "$mcpURL/" -Method Get -UseBasicParsing
    $rHead = Invoke-WebRequest -Uri "$mcpURL/" -Method Head -UseBasicParsing
    Write-Host "  -> GET Status  : $($rGet.StatusCode)" -ForegroundColor Green
    Write-Host "  -> HEAD Status : $($rHead.StatusCode)" -ForegroundColor Green
} catch {
    Write-Host "  -> FAILED: $_" -ForegroundColor Red
}

Write-Host "`n==========================================" -ForegroundColor Yellow
Write-Host " Audit Completed                          " -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Yellow
