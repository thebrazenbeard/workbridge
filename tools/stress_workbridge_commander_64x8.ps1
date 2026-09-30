param([string]$ResultPath='D:\VERA\.scratch\workbridge-stress-64x8-result.json')
$ErrorActionPreference='Stop'
$service=if($env:WORKBRIDGE_SERVICE_URL){$env:WORKBRIDGE_SERVICE_URL}else{'http://127.0.0.1:8787'}
$token=$env:WORKBRIDGE_CLIENT_TOKEN
if(-not $token){throw 'WORKBRIDGE_CLIENT_TOKEN is not available'}
$device='lappy';$contextCount=64;$effectSleepSec=6;$targetMs=[DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()+5000
$baseline=Invoke-RestMethod -Uri ($service+'/health') -TimeoutSec 5
$pool=[runspacefactory]::CreateRunspacePool(1,$contextCount);$pool.Open()
$worker={
 param($id,$target,$service,$device,$token,$sleepSec)
 while([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds() -lt $target){Start-Sleep -Milliseconds 1}
 $headers=@{Authorization=('Bearer '+$token);'x-workbridge-device'=$device}
 $cmd="Start-Sleep -Seconds $sleepSec; 'stress_context_$id=ok'"
 $arguments=@{command=$cmd;timeout_ms=20000;origin='llm'}
 $body=@{jsonrpc='2.0';id=$id;method='tools/call';params=@{name='start_process';arguments=$arguments}}|ConvertTo-Json -Depth 12 -Compress
 $sw=[Diagnostics.Stopwatch]::StartNew()
 try{
  $response=Invoke-RestMethod -Method Post -Uri ($service+'/mcp') -Headers $headers -ContentType 'application/json' -Body $body -TimeoutSec 120
  $sw.Stop();$serialized=$response.result.content|ConvertTo-Json -Depth 8 -Compress
  [pscustomobject]@{id=$id;ok=([bool]($serialized -match ("stress_context_"+$id+"=ok")));http_error=$null;duration_ms=$sw.ElapsedMilliseconds}
 }catch{
  $sw.Stop();[pscustomobject]@{id=$id;ok=$false;http_error=$_.Exception.Message;duration_ms=$sw.ElapsedMilliseconds}
 }
}
$items=@()
1..$contextCount|ForEach-Object{
 $ps=[powershell]::Create();$ps.RunspacePool=$pool
 [void]$ps.AddScript($worker.ToString()).AddArgument($_).AddArgument($targetMs).AddArgument($service).AddArgument($device).AddArgument($token).AddArgument($effectSleepSec)
 $items+=[pscustomobject]@{ps=$ps;handle=$ps.BeginInvoke()}
}
$peakUpstreamActive=0;$peakUpstreamQueued=0;$peakExecutionActive=0;$peakExecutionQueued=0;$samples=0
while(@($items|Where-Object{-not $_.handle.IsCompleted}).Count -gt 0){
 try{
  $h=Invoke-RestMethod -Uri ($service+'/health') -TimeoutSec 3
  $peakUpstreamActive=[math]::Max($peakUpstreamActive,[int]$h.upstreamContextActive)
  $peakUpstreamQueued=[math]::Max($peakUpstreamQueued,[int]$h.upstreamContextQueued)
  $peakExecutionActive=[math]::Max($peakExecutionActive,[int]$h.executionActive)
  $peakExecutionQueued=[math]::Max($peakExecutionQueued,[int]$h.executionQueued)
  $samples++
 }catch{}
 Start-Sleep -Milliseconds 100
}
$results=@()
foreach($item in $items){$results+=@($item.ps.EndInvoke($item.handle));$item.ps.Dispose()}
$pool.Close();$pool.Dispose()
$after=Invoke-RestMethod -Uri ($service+'/health') -TimeoutSec 5
$durations=@($results|ForEach-Object{[int]$_.duration_ms})
$summary=[ordered]@{
 schema='WORKBRIDGE_COMMANDER_STRESS_64X8_V1';target_ms=$targetMs;contexts=$contextCount;effect_sleep_seconds=$effectSleepSec
 configured_execution_capacity=[int]$baseline.executionCapacityPerDevice;configured_upstream_capacity=[int]$baseline.upstreamContextCapacity
 peak_upstream_active=$peakUpstreamActive;peak_upstream_queued=$peakUpstreamQueued;peak_execution_active=$peakExecutionActive;peak_execution_queued=$peakExecutionQueued
 health_samples=$samples;successful=@($results|Where-Object{$_.ok}).Count;failed=@($results|Where-Object{-not $_.ok}).Count
 min_duration_ms=($durations|Measure-Object -Minimum).Minimum;max_duration_ms=($durations|Measure-Object -Maximum).Maximum
 after_upstream_active=[int]$after.upstreamContextActive;after_upstream_queued=[int]$after.upstreamContextQueued
 after_execution_active=[int]$after.executionActive;after_execution_queued=[int]$after.executionQueued
 results=$results
}
$tmp=$ResultPath+'.tmp';$summary|ConvertTo-Json -Depth 8|Set-Content -Path $tmp -Encoding UTF8;Move-Item -Force $tmp $ResultPath
