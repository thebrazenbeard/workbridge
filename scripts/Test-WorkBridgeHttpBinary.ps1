[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Binary,
    [int]$TimeoutSeconds = 15
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$binaryPath = (Resolve-Path -LiteralPath $Binary).Path
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("workbridge-http-smoke-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null

$listener = New-Object Net.Sockets.TcpListener([Net.IPAddress]::Loopback, 0)
$listener.Start()
$port = ([Net.IPEndPoint]$listener.LocalEndpoint).Port
$listener.Stop()

$configPath = Join-Path $tempRoot "config.json"
$utf8NoBom = New-Object Text.UTF8Encoding -ArgumentList $false
$config = [ordered]@{
    schema = "WORKBRIDGE_CONFIG_V1"
    read_roots = @($tempRoot)
    write_roots = @()
    limits = [ordered]@{
        max_read_bytes = 1048576
        max_write_bytes = 1048576
        max_directory_entries = 500
    }
    process = [ordered]@{
        enabled = $false
        allowed_executables = @()
        working_roots = @()
        max_runtime_seconds = 60
        max_output_bytes = 1048576
        max_args = 64
    }
    http = [ordered]@{
        listen = "127.0.0.1:$port"
        path = "/mcp"
        bearer_token_env = "WORKBRIDGE_HTTP_TOKEN"
    }
}
$configJson = ($config | ConvertTo-Json -Depth 8) + [Environment]::NewLine
[IO.File]::WriteAllText($configPath, $configJson, $utf8NoBom)
$configBytes = [IO.File]::ReadAllBytes($configPath)
if ($configBytes.Length -ge 3 -and $configBytes[0] -eq 0xEF -and $configBytes[1] -eq 0xBB -and $configBytes[2] -eq 0xBF) {
    throw "HTTP smoke config unexpectedly contains a UTF-8 BOM"
}

$bytes = New-Object byte[] 48
$rng = [Security.Cryptography.RandomNumberGenerator]::Create()
try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
$token = [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+','-').Replace('/','_')

$psi = New-Object Diagnostics.ProcessStartInfo
$psi.FileName = $binaryPath
$psi.Arguments = '--config "' + $configPath.Replace('"','""') + '" --transport http'
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.CreateNoWindow = $true
$psi.EnvironmentVariables["WORKBRIDGE_HTTP_TOKEN"] = $token

$process = New-Object Diagnostics.Process
$process.StartInfo = $psi
$healthUri = "http://127.0.0.1:$port/mcp/healthz"

try {
    if (-not $process.Start()) {
        throw "Failed to start WorkBridgeMCP binary"
    }

    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $health = $null
    do {
        if ($process.HasExited) {
            $stderr = $process.StandardError.ReadToEnd()
            throw "WorkBridge exited before HTTP health was ready. exit=$($process.ExitCode) stderr=$stderr"
        }
        Start-Sleep -Milliseconds 100
        try {
            $health = Invoke-RestMethod -Uri $healthUri -Headers @{ Authorization = "Bearer $token" } -TimeoutSec 2
        } catch {
            $health = $null
        }
    } while ($null -eq $health -and [DateTime]::UtcNow -lt $deadline)

    if ($null -eq $health) {
        $stderr = $process.StandardError.ReadToEnd()
        throw "Authenticated HTTP health did not become ready within timeout. stderr=$stderr"
    }
    if ($health.status -ne "ok" -or [string]::IsNullOrWhiteSpace([string]$health.version)) {
        throw "Authenticated HTTP health response was invalid"
    }

    $unauthorized = $false
    try {
        Invoke-WebRequest -UseBasicParsing -Uri $healthUri -TimeoutSec 2 | Out-Null
    } catch {
        if ($_.Exception.Response -and [int]$_.Exception.Response.StatusCode -eq 401) {
            $unauthorized = $true
        } else {
            throw
        }
    }
    if (-not $unauthorized) {
        throw "HTTP health unexpectedly succeeded without bearer authentication"
    }

    [ordered]@{
        schema = "WORKBRIDGE_HTTP_BINARY_SMOKE_V1"
        status = "PASS"
        version = [string]$health.version
        listen = "127.0.0.1:$port"
        authenticated_health = "PASS"
        unauthenticated_health = "DENIED_401"
        process_enabled = $false
        config_bom_free = $true
    } | ConvertTo-Json -Depth 6
}
finally {
    if ($process -and -not $process.HasExited) {
        try { $process.Kill() } catch { }
    }
    if ($process) { $process.Dispose() }
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
