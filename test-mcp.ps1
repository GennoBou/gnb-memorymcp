# Remote MCP Endpoints & initialize Verification Script

$mcpURL = "https://oajuohz4tofchqb4hy62yzq65m0paryw.lambda-url.ap-northeast-3.on.aws"

Write-Host "=== 1. Testing Unauthenticated MCP initialize Endpoint ===" -ForegroundColor Cyan
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

$initResp = Invoke-RestMethod -Uri "$mcpURL/" -Method Post -ContentType "application/json" -Body $initReq
$initResp | ConvertTo-Json -Depth 5

Write-Host "`n=== 2. Testing Discovery Endpoint ===" -ForegroundColor Cyan
$meta = Invoke-RestMethod -Uri "$mcpURL/.well-known/oauth-authorization-server"
$meta | ConvertTo-Json

Write-Host "`n=== 3. Testing Token & tools/list Endpoint ===" -ForegroundColor Cyan
$tokenResp = Invoke-RestMethod -Uri "$mcpURL/token" -Method Post
$accessToken = $tokenResp.access_token

$listReq = @{
    jsonrpc = "2.0"
    method  = "tools/list"
    id      = 2
} | ConvertTo-Json

$listResp = Invoke-RestMethod -Uri "$mcpURL/" -Method Post -Headers @{ Authorization = "Bearer $accessToken" } -ContentType "application/json" -Body $listReq
Write-Host "Tools count: $($listResp.result.tools.Count)"
