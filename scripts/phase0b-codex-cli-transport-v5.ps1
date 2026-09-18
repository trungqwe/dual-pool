[CmdletBinding()]
param([string]$EvidenceDirectory)
$ErrorActionPreference = 'Stop'
if (-not $EvidenceDirectory) { $EvidenceDirectory = Join-Path $PSScriptRoot '../evidence/phase-0b-v5' }
$utf8 = New-Object Text.UTF8Encoding($false)
$started = [DateTime]::UtcNow
$runId = $started.ToString('yyyyMMddTHHmmssfffZ')
$testId = 'P0B-CX-TRANSPORT-001'
$tempBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$probeRoot = Join-Path $tempBase ('dual-pool-phase0b-v5-' + [Guid]::NewGuid().ToString('N'))
$probeHome = Join-Path $probeRoot 'home'
$work = Join-Path $probeRoot 'work'
$secret = 'SECRET_SENTINEL_' + [Guid]::NewGuid().ToString('N')
$prompt = 'PROMPT_SENTINEL_' + [Guid]::NewGuid().ToString('N')
$processes = New-Object Collections.Generic.List[object]
$environmentResult = @{}
$selfTestResult = @{ serializer = $false; fallback = $false; sse = $false }
$listenerResult = @{ observations = @(); loopback_owned = $false }
$requestResult = $null
$processResult = @{ codex_started = $false; codex_pid = $null; natural_exit = $false; timed_out = $false; killed_by_harness = $false; exit_code = $null; recorder_natural_exit = $false; recorder_exit_code = $null; classification = 'launch_failure' }
$securityResult = @{ scan_errors = 0; files_scanned = 0; secret_matches = 0; prompt_matches = 0; metadata_allowlist = $false; old_diagnostic_status = 'NOT_FOUND' }
$cleanupResult = @{ owned_processes_after = -1; listener_after = -1; temp_root_removed = $false; temporary_environment_unchanged = $false; old_diagnostic_absent = $false }
$failureResult = $null
$port = $null
$stage = 'initialize'
$envBefore = [Environment]::GetEnvironmentVariable('DUALPOOL_CODEX_KEY', 'Process')
$homeBefore = [Environment]::GetEnvironmentVariable('CODEX_HOME', 'Process')
$ownedConfigs = @((Join-Path $env:USERPROFILE '.codex/config.toml'), (Join-Path $env:APPDATA 'Antigravity IDE/User/settings.json'))
function Snapshot-Configs {
    @($ownedConfigs | ForEach-Object {
        if (Test-Path -LiteralPath $_) { (Get-FileHash -LiteralPath $_ -Algorithm SHA256).Hash } else { 'ABSENT' }
    })
}
$configBefore = Snapshot-Configs
function Write-Json([string]$Path, $Value) {
    $json = $Value | ConvertTo-Json -Depth 40
    if ($json.Contains($secret) -or $json.Contains($prompt)) { throw 'SENTINEL_IN_RESULT' }
    [IO.File]::WriteAllText($Path, $json + "`n", $utf8)
    $read = [IO.File]::ReadAllText($Path) | ConvertFrom-Json
    if ($read.test_id -ne $testId -or $read.run_id -ne $runId -or $read.status -ne $Value.status) { throw 'RESULT_READBACK_MISMATCH' }
}
function Write-Fallback([string]$Path, [string]$ExceptionType) {
    Write-Json $Path @{ schema_version = '5'; test_id = $testId; run_id = $runId; status = 'FAIL'; failure_classification = 'finalizer_failure'; exception_type = $ExceptionType; stage = 'final_write'; cleanup = $cleanupResult; timestamp = [DateTime]::UtcNow.ToString('o') }
}
function Start-Owned([string]$Exe, [string[]]$Arguments, [hashtable]$ChildEnvironment) {
    $info = New-Object Diagnostics.ProcessStartInfo
    $info.FileName = $Exe
    $info.Arguments = ($Arguments | ForEach-Object { '"' + ($_ -replace '"', '\"') + '"' }) -join ' '
    $info.WorkingDirectory = $work
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $info.RedirectStandardInput = $true
    foreach ($key in $ChildEnvironment.Keys) { $info.EnvironmentVariables[$key] = $ChildEnvironment[$key] }
    $p = New-Object Diagnostics.Process
    $p.StartInfo = $info
    if (-not $p.Start()) { throw 'PROCESS_START_FAILED' }
    $entry = @{ process = $p; pid = $p.Id; start_ticks = $p.StartTime.ToUniversalTime().Ticks; image = $Exe; stdout = $p.StandardOutput.ReadToEndAsync(); stderr = $p.StandardError.ReadToEndAsync() }
    $processes.Add($entry)
    return $entry
}
function Stop-Owned($Entry) {
    $p = $Entry.process
    if ($p.HasExited) { return }
    $current = Get-Process -Id $Entry.pid -ErrorAction Stop
    if ($current.StartTime.ToUniversalTime().Ticks -ne $Entry.start_ticks -or $current.Path -ne $Entry.image) { throw 'PROCESS_IDENTITY_MISMATCH' }
    $p.Kill()
    if (-not $p.WaitForExit(5000)) { throw 'PROCESS_CLEANUP_TIMEOUT' }
}
function Pipe-Metadata($Entry) {
    $data = @{}
    foreach ($name in @('stdout','stderr')) {
        $task = $Entry[$name]
        if (-not $task.Wait(5000)) { throw 'PIPE_DRAIN_TIMEOUT' }
        $value = $task.Result
        $data[$name + '_nonempty'] = $value.Length -gt 0
        $data[$name + '_length_bucket'] = if ($value.Length -eq 0) { '0' } elseif ($value.Length -le 4096) { '1-4096' } elseif ($value.Length -le 1048576) { '4097-1048576' } else { 'OVER_LIMIT' }
        $data['secret_sentinel_seen_in_' + $name] = $value.Contains($secret)
        $data['prompt_sentinel_seen_in_' + $name] = $value.Contains($prompt)
        if ($value.Length -gt 1048576) { throw 'PIPE_OUTPUT_LIMIT' }
    }
    return $data
}
try {
    New-Item -ItemType Directory -Path $probeHome,$work -Force | Out-Null
    $stage = 'diagnostic_inspection'
    $legacy = Join-Path $tempBase 'dual-pool-phase0b-v4-diagnostic.txt'
    if (Test-Path -LiteralPath $legacy) {
        $raw = [IO.File]::ReadAllText($legacy)
        $securityResult.old_diagnostic_status = 'FOUND_SANITIZED_AND_REMOVED'
        $securityResult.old_diagnostic_async_runspace_marker = $raw -match 'Runspace|OutputDataReceived|ErrorDataReceived'
        Remove-Item -LiteralPath $legacy -Force
    }
    $stage = 'serializer_selftest'
    $representative = @{schema_version='5';test_id=$testId;run_id=$runId;status='FAIL';process=@{natural_exit=$false;exit_code=$null};assertions=@('a');cleanup=@{removed=$true}}
    Write-Json (Join-Path $probeRoot 'serializer.json') $representative
    $selfTestResult.serializer = $true
    # Force a primary write failure by targeting a directory, then exercise fallback.
    try {
        Write-Json $probeRoot $representative
    } catch {
        Write-Fallback (Join-Path $probeRoot 'fallback.json') $_.Exception.GetType().FullName
        $fallbackRead = Get-Content -Raw (Join-Path $probeRoot 'fallback.json') | ConvertFrom-Json
        $selfTestResult.fallback = $fallbackRead.failure_classification -eq 'finalizer_failure'
    }
    if (-not $selfTestResult.fallback) { throw 'FINALIZER_SELFTEST_FAIL' }
    $codexExe = (Get-Command codex.exe -ErrorAction Stop).Source
    $nodeExe = (Get-Command node.exe -ErrorAction Stop).Source
    $environmentResult = @{ os = [Environment]::OSVersion.VersionString; architecture = 'x64'; codex_version = (& $codexExe --version | Out-String).Trim(); codex_sha256 = (Get-FileHash $codexExe -Algorithm SHA256).Hash; node_version = (& $nodeExe --version | Out-String).Trim(); node_sha256 = (Get-FileHash $nodeExe -Algorithm SHA256).Hash }
    $stage = 'recorder_start'
    $helper = Join-Path $PSScriptRoot 'phase0b-recorder-v5.cjs'
    $recorder = Start-Owned $nodeExe @($helper) @{P0B_ROOT=$probeRoot;P0B_SECRET=$secret;P0B_PROMPT=$prompt}
    $recorder.process.StandardInput.Close()
    $readyFile = Join-Path $probeRoot 'ready.json'
    $deadline = [DateTime]::UtcNow.AddSeconds(5)
    while (-not (Test-Path $readyFile) -and [DateTime]::UtcNow -lt $deadline -and -not $recorder.process.HasExited) { Start-Sleep -Milliseconds 50 }
    $ready = Get-Content -Raw $readyFile | ConvertFrom-Json
    $port = [int]$ready.port
    $stage = 'listener_verification'
    $listeners = @(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction Stop)
    $listenerResult.observations = @($listeners | ForEach-Object { @{address=$_.LocalAddress;port=[int]$_.LocalPort;pid_owned=($_.OwningProcess -eq $recorder.pid)} })
    $listenerResult.loopback_owned = $listeners.Count -eq 1 -and $listeners[0].LocalAddress -eq '127.0.0.1' -and $listeners[0].OwningProcess -eq $recorder.pid
    if (-not $listenerResult.loopback_owned) { throw 'LISTENER_ASSERTION_FAILED' }
    $stage = 'sse_http_selftest'
    $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$port/selftest" -TimeoutSec 5
    $frames = @($response.Content -split "`n`n" | Where-Object { $_ })
    $types = @($frames | ForEach-Object { ($_.Substring(6) | ConvertFrom-Json).type })
    $selfTestResult.sse = -not $ready.old_has_lf_lf -and $ready.new_has_lf_lf -and $response.Headers['Content-Type'] -eq 'text/event-stream' -and ($types -join ',') -eq 'response.created,response.completed'
    $selfTestResult.old_has_consecutive_lf = $ready.old_has_lf_lf
    $selfTestResult.new_has_consecutive_lf = $ready.new_has_lf_lf
    $selfTestResult.http_eof_received = $true
    if (-not $selfTestResult.sse) { throw 'HARNESS_SELFTEST_FAIL' }
    $stage = 'codex_start'
    $config = @"
model = "gpt-6-astra"
model_provider = "dualpool_codex"
[model_providers.dualpool_codex]
name = "Dual Pool Probe"
base_url = "http://127.0.0.1:$port/v1"
wire_api = "responses"
env_key = "DUALPOOL_CODEX_KEY"
"@
    [IO.File]::WriteAllText((Join-Path $probeHome 'config.toml'), $config, $utf8)
    $client = Start-Owned $codexExe @('exec','--strict-config','--ephemeral','--json','--skip-git-repo-check','--sandbox','read-only','-') @{CODEX_HOME=$probeHome;DUALPOOL_CODEX_KEY=$secret}
    $processResult.codex_started = $true
    $processResult.codex_pid = $client.pid
    $client.process.StandardInput.WriteLine('Synthetic transport probe: ' + $prompt)
    $client.process.StandardInput.Close()
    $stage = 'codex_wait'
    $processResult.natural_exit = $client.process.WaitForExit(15000)
    $processResult.timed_out = -not $processResult.natural_exit
    if ($processResult.natural_exit) {
        $client.process.WaitForExit()
        $processResult.exit_code = $client.process.ExitCode
        $processResult.classification = if ($client.process.ExitCode -eq 0) {'natural_success'} else {'natural_nonzero'}
    } else {
        Stop-Owned $client
        $processResult.killed_by_harness = $true
        $processResult.exit_code = $client.process.ExitCode
        $processResult.classification = 'timeout_forced_cleanup'
    }
    $processResult.recorder_natural_exit = $recorder.process.WaitForExit(5000)
    if ($processResult.recorder_natural_exit) { $processResult.recorder_exit_code = $recorder.process.ExitCode }
    $processResult.pipes = Pipe-Metadata $client
    $stage = 'capture_validation'
    $captureFile = Join-Path $probeRoot 'capture.json'
    $requestResult = Get-Content -Raw $captureFile | ConvertFrom-Json
    $allow = @('method','route','content_type','header_names','model','request_shape','authorization_header_present','secret_seen_at_expected_ingress','prompt_seen_at_expected_ingress','response_content_type','emitted_events','actual_lf_lf','response_closed')
    $securityResult.metadata_allowlist = @($requestResult.PSObject.Properties.Name | Where-Object { $_ -notin $allow }).Count -eq 0
} catch {
    $failureResult = @{ classification='harness_runtime_failure'; stage=$stage; exception_type=$_.Exception.GetType().FullName; category=[string]$_.CategoryInfo.Category; script_line=$_.InvocationInfo.ScriptLineNumber }
} finally {
    foreach ($entry in $processes) {
        try { Stop-Owned $entry } catch { $failureResult = @{classification='cleanup_failure';stage='stop_owned';exception_type=$_.Exception.GetType().FullName} }
    }
    try {
        $files = @(Get-ChildItem -LiteralPath $probeRoot -File -Recurse -Force)
        foreach ($file in $files) {
            try {
                $bytes = [IO.File]::ReadAllBytes($file.FullName)
                $securityResult.files_scanned++
                $text = [Text.Encoding]::UTF8.GetString($bytes)
                if ($text.Contains($secret)) { $securityResult.secret_matches++ }
                if ($text.Contains($prompt)) { $securityResult.prompt_matches++ }
            } catch { $securityResult.scan_errors++ }
        }
        $resolved = [IO.Path]::GetFullPath($probeRoot)
        if (-not $resolved.StartsWith($tempBase, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $resolved -Leaf) -notmatch '^dual-pool-phase0b-v5-[a-f0-9]{32}$') { throw 'UNSAFE_TEMP_TARGET' }
        Remove-Item -LiteralPath $resolved -Recurse -Force
        $cleanupResult.temp_root_removed = -not (Test-Path -LiteralPath $resolved)
        $cleanupResult.owned_processes_after = @($processes | Where-Object { -not $_.process.HasExited }).Count
        $cleanupResult.listener_after = if ($null -eq $port) {0} else {@(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue).Count}
        $cleanupResult.temporary_environment_unchanged = [Environment]::GetEnvironmentVariable('DUALPOOL_CODEX_KEY','Process') -ceq $envBefore -and [Environment]::GetEnvironmentVariable('CODEX_HOME','Process') -ceq $homeBefore
        $cleanupResult.old_diagnostic_absent = -not (Test-Path (Join-Path $tempBase 'dual-pool-phase0b-v4-diagnostic.txt'))
        $cleanupResult.user_config_before = $configBefore
        $cleanupResult.user_config_after = Snapshot-Configs
        $cleanupResult.user_configs_unchanged = ($configBefore -join ',') -eq ($cleanupResult.user_config_after -join ',')
    } catch { $failureResult = @{classification='cleanup_failure';stage='scan_remove_verify';exception_type=$_.Exception.GetType().FullName} }
}
# Operational objects never enter the serialized graph. Missing observations fail closed.
$assertions = [ordered]@{
    serializer = $selfTestResult.serializer
    fallback = $selfTestResult.fallback
    sse = $selfTestResult.sse
    listener_owned_loopback = $listenerResult.loopback_owned
    request = ($null -ne $requestResult -and $requestResult.method -eq 'POST' -and $requestResult.route -eq '/v1/responses' -and $requestResult.model -eq 'gpt-6-astra' -and $requestResult.content_type -match '^application/json')
    ingress = ($null -ne $requestResult -and $requestResult.authorization_header_present -and $requestResult.secret_seen_at_expected_ingress -and $requestResult.prompt_seen_at_expected_ingress)
    shape = ($null -ne $requestResult -and 'body.model:string' -in $requestResult.request_shape -and 'body.input:array' -in $requestResult.request_shape -and 'body.stream:boolean' -in $requestResult.request_shape -and 'body.reasoning:object' -in $requestResult.request_shape -and 'body.parallel_tool_calls:boolean' -in $requestResult.request_shape -and @($requestResult.request_shape | Where-Object {$_ -match '\.tools:array$'}).Count -gt 0)
    response = ($null -ne $requestResult -and $requestResult.response_closed -and $requestResult.actual_lf_lf -and ($requestResult.emitted_events -join ',') -eq 'response.created,response.completed')
    codex = ($processResult.natural_exit -and -not $processResult.timed_out -and -not $processResult.killed_by_harness -and $processResult.exit_code -eq 0)
    recorder = ($processResult.recorder_natural_exit -and $processResult.recorder_exit_code -eq 0)
    memory_pipes = ($null -ne $processResult.pipes -and -not $processResult.pipes.secret_sentinel_seen_in_stdout -and -not $processResult.pipes.secret_sentinel_seen_in_stderr)
    security = ($securityResult.scan_errors -eq 0 -and $securityResult.secret_matches -eq 0 -and $securityResult.prompt_matches -eq 0 -and $securityResult.metadata_allowlist)
    cleanup = ($cleanupResult.temp_root_removed -and $cleanupResult.owned_processes_after -eq 0 -and $cleanupResult.listener_after -eq 0 -and $cleanupResult.temporary_environment_unchanged -and $cleanupResult.old_diagnostic_absent -and $cleanupResult.user_configs_unchanged)
    no_runtime_failure = ($null -eq $failureResult)
}
$status = if (@($assertions.Values | Where-Object {$_ -ne $true}).Count -eq 0) {'PASS'} else {'FAIL'}
$finalResult = @{schema_version='5';test_id=$testId;run_id=$runId;status=$status;started_at=$started.ToString('o');ended_at=[DateTime]::UtcNow.ToString('o');duration_ms=[int]([DateTime]::UtcNow-$started).TotalMilliseconds;environment=$environmentResult;self_test=$selfTestResult;listener=$listenerResult;request=$requestResult;process=$processResult;security=$securityResult;cleanup=$cleanupResult;failure=$failureResult;assertions=$assertions;timeout_ms=15000;command_shape='codex exec --strict-config --ephemeral --json --skip-git-repo-check --sandbox read-only -';configuration_mechanism='isolated CODEX_HOME/config.toml; child-only environment; synthetic stdin';limitations=@('Windows 10 exploratory; extension/config-layer and Windows 11 acceptance remain open.');finalizer=@{primary_writer_selftest=$selfTestResult.serializer;fallback_writer_selftest=$selfTestResult.fallback}}
New-Item -ItemType Directory -Path $EvidenceDirectory -Force | Out-Null
$target = Join-Path $EvidenceDirectory ('codex-cli-transport-v5-' + $runId + '.json')
try { Write-Json $target $finalResult } catch {
    try { Write-Fallback $target $_.Exception.GetType().FullName; $status='FAIL' } catch { Write-Output 'FILESYSTEM_EVIDENCE_FAILURE'; exit 3 }
}
$hash = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash
Write-Output ("RESULT {0} {1} SHA256={2}" -f (Split-Path $target -Leaf),$status,$hash)
if ($status -ne 'PASS') { exit 1 }
