[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Binary,
    [string]$ProtocolVersion = "2026-07-28",
    [int]$TimeoutSeconds = 10
)

$ErrorActionPreference = "Stop"
$binaryPath = (Resolve-Path -LiteralPath $Binary).Path
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("workbridge-smoke-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
$configPath = Join-Path $tempRoot "config.json"

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
        listen = "127.0.0.1:8765"
        path = "/mcp"
        bearer_token_env = ""
    }
}
$configJson = ($config | ConvertTo-Json -Depth 8) + [Environment]::NewLine
$utf8NoBom = New-Object System.Text.UTF8Encoding -ArgumentList $false
[IO.File]::WriteAllText($configPath, $configJson, $utf8NoBom)
$configBytes = [IO.File]::ReadAllBytes($configPath)
if ($configBytes.Length -ge 3 -and $configBytes[0] -eq 0xEF -and $configBytes[1] -eq 0xBB -and $configBytes[2] -eq 0xBF) {
    throw "Smoke config unexpectedly contains a UTF-8 BOM"
}

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $binaryPath
$psi.Arguments = '--config "' + $configPath.Replace('"', '""') + '"'
$psi.UseShellExecute = $false
$psi.RedirectStandardInput = $true
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.StandardInputEncoding = $utf8NoBom
$psi.CreateNoWindow = $true

$process = New-Object System.Diagnostics.Process
$process.StartInfo = $psi

function Write-JsonRpcLine {
    param(
        [Parameter(Mandatory=$true)]$Process,
        [Parameter(Mandatory=$true)][string]$Line
    )

    $Process.StandardInput.WriteLine($Line)
    $Process.StandardInput.Flush()
}

function Read-JsonRpcResponse {
    param(
        [Parameter(Mandatory=$true)]$Process,
        [Parameter(Mandatory=$true)][int]$Id,
        [Parameter(Mandatory=$true)][DateTime]$Deadline
    )

    while ([DateTime]::UtcNow -lt $Deadline -and -not $Process.HasExited) {
        if ($Process.StandardOutput.Peek() -ge 0) {
            $line = $Process.StandardOutput.ReadLine()
            if (-not [string]::IsNullOrWhiteSpace($line)) {
                $candidate = $line | ConvertFrom-Json
                if ($candidate.id -eq $Id) {
                    return $candidate
                }
            }
        } else {
            Start-Sleep -Milliseconds 50
        }
    }
    return $null
}

try {
    if (-not $process.Start()) {
        throw "Failed to start WorkBridgeMCP binary"
    }

    $initialize = @{
        jsonrpc = "2.0"
        id = 1
        method = "initialize"
        params = @{
            protocolVersion = $ProtocolVersion
            capabilities = @{}
            clientInfo = @{
                name = "workbridge-blackbox-smoke"
                version = "1.0"
            }
        }
    } | ConvertTo-Json -Compress -Depth 8

    Write-JsonRpcLine -Process $process -Line $initialize

    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $response = Read-JsonRpcResponse -Process $process -Id 1 -Deadline $deadline

    if ($null -eq $response) {
        $stderr = $process.StandardError.ReadToEnd()
        throw "No initialize response received within timeout. stderr: $stderr"
    }
    if ($null -ne $response.error) {
        throw ("Initialize returned JSON-RPC error: " + ($response.error | ConvertTo-Json -Compress))
    }
    if ($null -eq $response.result.protocolVersion) {
        throw "Initialize response omitted protocolVersion"
    }

    $initialized = @{
        jsonrpc = "2.0"
        method = "notifications/initialized"
        params = @{}
    } | ConvertTo-Json -Compress -Depth 4
    Write-JsonRpcLine -Process $process -Line $initialized

    $toolsList = @{
        jsonrpc = "2.0"
        id = 2
        method = "tools/list"
        params = @{}
    } | ConvertTo-Json -Compress -Depth 4
    Write-JsonRpcLine -Process $process -Line $toolsList

    $toolsResponse = Read-JsonRpcResponse -Process $process -Id 2 -Deadline ([DateTime]::UtcNow.AddSeconds($TimeoutSeconds))
    if ($null -eq $toolsResponse) {
        throw "No tools/list response received within timeout"
    }
    if ($null -ne $toolsResponse.error) {
        throw ("tools/list returned JSON-RPC error: " + ($toolsResponse.error | ConvertTo-Json -Compress))
    }

    $toolNames = @($toolsResponse.result.tools | ForEach-Object { $_.name })
    foreach ($requiredTool in @("workbridge_health", "workspace_list", "workspace_stat", "workspace_read_text")) {
        if ($toolNames -notcontains $requiredTool) {
            throw "tools/list omitted required read-only tool: $requiredTool"
        }
    }
    foreach ($forbiddenTool in @("workspace_write_text", "workspace_mkdir", "process_run")) {
        if ($toolNames -contains $forbiddenTool) {
            throw "read-only smoke profile exposed disabled tool: $forbiddenTool"
        }
    }

    Write-Output ([ordered]@{
        initialize = "PASS"
        toolsList = "PASS"
        negotiatedProtocolVersion = $response.result.protocolVersion
        serverInfo = $response.result.serverInfo
        tools = $toolNames
    } | ConvertTo-Json -Depth 8)
}
finally {
    if ($process -and -not $process.HasExited) {
        try { $process.Kill() } catch { }
    }
    if ($process) { $process.Dispose() }
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
}
