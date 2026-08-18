$baseUrl = "https://oajuohz4tofchqb4hy62yzq65m0paryw.lambda-url.ap-northeast-3.on.aws"

$testPaths = @(
    "",
    "/",
    "/.well-known/oauth-authorization-server",
    "/.well-known/openid-configuration",
    "/.well-known/mcp-configuration",
    "/.well-known/mcp"
)

$methods = @("GET", "HEAD", "OPTIONS")

foreach ($p in $testPaths) {
    foreach ($m in $methods) {
        $target = "$baseUrl$p"
        try {
            $resp = Invoke-WebRequest -Uri $target -Method $m -UseBasicParsing -ErrorAction Stop
            Write-Host "[$m] $target -> Status: $($resp.StatusCode), Headers: $($resp.Headers['Access-Control-Allow-Origin'])" -ForegroundColor Green
        } catch {
            Write-Host "[$m] $target -> ERROR: $_" -ForegroundColor Red
        }
    }
}
