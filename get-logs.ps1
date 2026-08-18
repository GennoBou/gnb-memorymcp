$startTime = [DateTimeOffset]::Now.AddMinutes(-5).ToUnixTimeMilliseconds()
$logGroup = "/aws/lambda/gnb-memorymcp"
$region = "ap-northeast-3"

$json = aws logs filter-log-events --log-group-name $logGroup --start-time $startTime --region $region
$data = $json | ConvertFrom-Json

Write-Host "=== Total Log Events: $($data.events.Count) ===" -ForegroundColor Yellow
foreach ($ev in $data.events) {
    $time = [DateTimeOffset]::FromUnixTimeMilliseconds($ev.timestamp).ToString("yyyy-MM-dd HH:mm:ss.fff")
    Write-Host "[$time] $($ev.message)"
}
