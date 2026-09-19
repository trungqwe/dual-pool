[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$SessionRoot,
    [string]$EvidenceDirectory,
    [int]$TimeoutMinutes = 15
)

$ErrorActionPreference = 'Stop'
$utf8 = New-Object Text.UTF8Encoding($false)
$repoRoot = Split-Path $PSScriptRoot -Parent
if (-not $EvidenceDirectory) { $EvidenceDirectory = Join-Path $repoRoot 'evidence/phase-0b-codex-extension' }
$tempBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$resolvedSession = [IO.Path]::GetFullPath($SessionRoot)
if (-not $resolvedSession.StartsWith($tempBase, [StringComparison]::OrdinalIgnoreCase) -or
    (Split-Path $resolvedSession -Leaf) -notmatch '^dual-pool-extension-probe-[a-f0-9]{32}$') { throw 'UNSAFE_SESSION_ROOT' }

$started = [DateTime]::UtcNow
$runId = $started.ToString('yyyyMMddTHHmmssfffZ')
$testId = 'P0B-CX-EXTENSION-TWO-REQUEST-001'
$ideUserData = Join-Path $resolvedSession 'ide-user-data'
$codexHome = Join-Path $resolvedSession 'codex-home'
$extensionSilo = Join-Path $resolvedSession 'extensions'
$workspace = Join-Path $resolvedSession 'workspace'
$recorderRoot = Join-Path $resolvedSession 'recorder'
$controlRoot = Join-Path $resolvedSession 'state/control'
$stateRoot = Join-Path $resolvedSession 'state'
$statusPath = Join-Path $stateRoot 'status.json'
$baselinePromptPath = Join-Path $stateRoot 'baseline-prompt.txt'
$astraPromptPath = Join-Path $stateRoot 'astra-prompt.txt'
$secret = 'SECRET_SENTINEL_' + [Guid]::NewGuid().ToString('N')
$baselinePrompt = 'BASELINE_PROMPT_SENTINEL_' + [Guid]::NewGuid().ToString('N')
$astraPrompt = 'ASTRA_PROMPT_SENTINEL_' + [Guid]::NewGuid().ToString('N')
$initialModel = 'gpt-5.6-sol'
$events = New-Object Collections.Generic.List[object]
$protectedPids = @()
$probeRootPid = $null
$probePids = @()
$recorder = $null
$port = $null
$capture1 = $null
$capture2 = $null
$failure = $null
$classification = 'RUNNING'
$status = 'BLOCKED'
$userBaseline = $null
$userAstra = $null
$deadline = $started.AddMinutes($TimeoutMinutes)
$realConfig = Join-Path $env:USERPROFILE '.codex/config.toml'
$realSettings = Join-Path $env:APPDATA 'Antigravity IDE/User/settings.json'

function Safe-Hash([string]$Path) {
    if (Test-Path -LiteralPath $Path -PathType Leaf) { return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash }
    return 'ABSENT'
}
function Write-SafeJson([string]$Path, $Value) {
    $json = $Value | ConvertTo-Json -Depth 40
    if ($json.Contains($secret) -or $json.Contains($baselinePrompt) -or $json.Contains($astraPrompt)) { throw 'SENTINEL_IN_EVIDENCE' }
    [IO.File]::WriteAllText($Path, $json + "`n", $utf8)
}
function Write-State([string]$Stage, [hashtable]$Metadata) {
    Write-SafeJson $statusPath @{run_id=$runId;test_id=$testId;stage=$Stage;updated_at=[DateTime]::UtcNow.ToString('o');metadata=$Metadata}
}
function Add-Event([string]$Type, [hashtable]$Metadata) {
    $events.Add([ordered]@{timestamp=[DateTime]::UtcNow.ToString('o');event_type=$Type;metadata=$Metadata})
    New-Item -ItemType Directory -Path $EvidenceDirectory -Force | Out-Null
    Write-SafeJson (Join-Path $EvidenceDirectory ('probe-events-' + $runId + '.json')) @{schema_version='1';test_id=$testId;run_id=$runId;events=@($events | ForEach-Object {$_})}
}
function Probe-Processes {
    @(Get-CimInstance Win32_Process | Where-Object {$_.Name -eq 'Antigravity IDE.exe' -and $_.CommandLine -like ('*' + $ideUserData + '*')})
}
function Originals-Alive {
    @($protectedPids | Where-Object { -not (Get-Process -Id $_ -ErrorAction SilentlyContinue) }).Count -eq 0
}
function Wait-Control([string[]]$Allowed, [string]$Stage) {
    Write-State $Stage @{waiting=$true;allowed_controls=$Allowed}
    while ([DateTime]::UtcNow -lt $deadline) {
        foreach ($name in $Allowed) { if (Test-Path -LiteralPath (Join-Path $controlRoot $name)) { return $name } }
        Start-Sleep -Milliseconds 500
    }
    throw 'USER_ASSISTED_PROBE_TIMEOUT'
}
function Wait-Capture([int]$Ordinal) {
    $file = Join-Path $recorderRoot ('capture-' + $Ordinal + '.json')
    while (-not (Test-Path -LiteralPath $file) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 250 }
    if (-not (Test-Path -LiteralPath $file)) { throw 'USER_ASSISTED_PROBE_TIMEOUT' }
    return Get-Content -Raw -LiteralPath $file | ConvertFrom-Json
}
function Capture-Pass($Capture, [int]$Ordinal, [string]$Model, [string]$PromptKind) {
    $promptSeen = if ($PromptKind -eq 'baseline') {$Capture.baseline_sentinel_seen_at_expected_ingress} else {$Capture.astra_sentinel_seen_at_expected_ingress}
    return $Capture.ordinal -eq $Ordinal -and $Capture.method -eq 'POST' -and $Capture.route -eq '/v1/responses' -and $Capture.model -eq $Model -and $Capture.content_type -match '^application/json' -and $Capture.secret_seen_at_expected_ingress -and $promptSeen -and $Capture.expected_prompt_seen_at_expected_ingress -and $Capture.response_closed
}

$realConfigBefore = Safe-Hash $realConfig
$realSettingsBefore = Safe-Hash $realSettings
$extensionManifestBefore = 'ABSENT'
$extensionManifestAfter = 'ABSENT'
$sourceExtension = $null
$extensionPackage = $null
$flags = @{}
$recorderReady = $false
$smokePass = $false
$extensionIsolationPass = $false

try {
    New-Item -ItemType Directory -Path $ideUserData,$codexHome,$extensionSilo,$workspace,$recorderRoot,$controlRoot,$stateRoot,$EvidenceDirectory -Force | Out-Null
    Write-State 'PRECHECK' @{started=$true}
    $ideRoot = Join-Path $env:LOCALAPPDATA 'Programs/Antigravity IDE'
    $ideExe = Join-Path $ideRoot 'Antigravity IDE.exe'
    $cliScript = Join-Path $ideRoot 'resources/app/out/cli.js'
    $cliCmd = Join-Path $ideRoot 'bin/antigravity-ide.cmd'
    $extensionsRoot = Join-Path $env:USERPROFILE '.antigravity-ide/extensions'
    $nodeExe = (Get-Command node.exe -ErrorAction Stop).Source
    foreach ($required in @($ideExe,$cliScript,$cliCmd,$extensionsRoot,$nodeExe)) { if (-not (Test-Path -LiteralPath $required)) { throw 'REQUIRED_INSTALLATION_PATH_ABSENT' } }

    $helpText = (& $cliCmd --help 2>&1 | Out-String)
    $flags = @{user_data_dir=$helpText.Contains('--user-data-dir');extensions_dir=$helpText.Contains('--extensions-dir');new_window=$helpText.Contains('--new-window')}
    if (-not $flags.user_data_dir) { throw 'PARALLEL_USER_DATA_UNSUPPORTED' }
    $catalog = (& codex debug models --bundled | ConvertFrom-Json).models
    if (@($catalog | Where-Object {$_.slug -eq $initialModel -and $_.visibility -eq 'list'}).Count -ne 1 -or @($catalog | Where-Object {$_.slug -eq 'gpt-6-astra' -and $_.visibility -eq 'list'}).Count -ne 1) { throw 'BUNDLED_MODEL_PRECHECK_FAILED' }
    $sourceExtension = @(Get-ChildItem -LiteralPath $extensionsRoot -Directory | Where-Object {$_.Name -like 'openai.chatgpt-*'})
    if ($sourceExtension.Count -ne 1) { throw 'OPENAI_EXTENSION_DISCOVERY_FAILED' }
    $extensionPackage = Get-Content -Raw (Join-Path $sourceExtension[0].FullName 'package.json') | ConvertFrom-Json
    $extensionManifestBefore = Safe-Hash (Join-Path $sourceExtension[0].FullName 'package.json')
    $copyTarget = Join-Path $extensionSilo $sourceExtension[0].Name
    Copy-Item -LiteralPath $sourceExtension[0].FullName -Destination $copyTarget -Recurse -Force
    $extensionManifestAfter = Safe-Hash (Join-Path $copyTarget 'package.json')
    if ($extensionManifestBefore -ne $extensionManifestAfter) { throw 'EXTENSION_SILO_HASH_MISMATCH' }
    $extensionIsolationPass = $true
    $protectedPids = @(Get-CimInstance Win32_Process | Where-Object {$_.Name -eq 'Antigravity IDE.exe'} | Select-Object -ExpandProperty ProcessId)
    if ($protectedPids.Count -eq 0) { throw 'ORIGINAL_IDE_NOT_RUNNING' }
    Add-Event 'PRECHECK_PASS' @{flags=$flags;initial_model_observed=$true;astra_observed=$true;protected_process_count=$protectedPids.Count}
    Add-Event 'PARALLEL_SMOKE_PASS' @{different_process_tree_pending=$true;temporary_user_data_ready=$true;original_instance_alive=(Originals-Alive)}
    Add-Event 'EXTENSION_ISOLATION_PASS' @{temporary_extension_silo=$true;manifest_hash_match=$true}

    $recorderScript = Join-Path $PSScriptRoot 'phase0b-extension-recorder.cjs'
    $recorderInfo = [Diagnostics.ProcessStartInfo]::new()
    $recorderInfo.FileName = $nodeExe; $recorderInfo.Arguments = '"' + $recorderScript + '"'; $recorderInfo.UseShellExecute = $false; $recorderInfo.CreateNoWindow = $true
    foreach ($pair in @{P0B_ROOT=$recorderRoot;P0B_SECRET=$secret;P0B_BASELINE_PROMPT=$baselinePrompt;P0B_ASTRA_PROMPT=$astraPrompt;P0B_INITIAL_MODEL=$initialModel}.GetEnumerator()) { $recorderInfo.EnvironmentVariables[$pair.Key] = $pair.Value }
    $recorder = [Diagnostics.Process]::Start($recorderInfo)
    $readyDeadline = [DateTime]::UtcNow.AddSeconds(10)
    while (-not (Test-Path -LiteralPath (Join-Path $recorderRoot 'ready.json')) -and [DateTime]::UtcNow -lt $readyDeadline -and -not $recorder.HasExited) { Start-Sleep -Milliseconds 100 }
    if (-not (Test-Path -LiteralPath (Join-Path $recorderRoot 'ready.json'))) { throw 'RECORDER_NOT_READY' }
    $ready = Get-Content -Raw (Join-Path $recorderRoot 'ready.json') | ConvertFrom-Json
    $port = [int]$ready.port
    $listeners = @(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction Stop)
    if ($listeners.Count -ne 1 -or $listeners[0].LocalAddress -ne '127.0.0.1' -or $listeners[0].OwningProcess -ne $recorder.Id) { throw 'RECORDER_OWNERSHIP_FAILED' }
    $recorderReady = $true
    Add-Event 'RECORDER_READY' @{loopback_only=$true;pid_owned=$true;two_request_target=$true}

    $config = @"
model = "$initialModel"
model_provider = "dualpool_probe"

[model_providers.dualpool_probe]
name = "Dual Pool Phase 0B"
base_url = "http://127.0.0.1:$port/v1"
wire_api = "responses"
env_key = "DUALPOOL_CODEX_KEY"
"@
    [IO.File]::WriteAllText((Join-Path $codexHome 'config.toml'), $config, $utf8)
    $launchInfo = [Diagnostics.ProcessStartInfo]::new()
    $launchInfo.FileName = $ideExe; $launchInfo.UseShellExecute = $false; $launchInfo.EnvironmentVariables['ELECTRON_RUN_AS_NODE']='1'; $launchInfo.EnvironmentVariables['VSCODE_DEV']=''; $launchInfo.EnvironmentVariables['CODEX_HOME']=$codexHome; $launchInfo.EnvironmentVariables['DUALPOOL_CODEX_KEY']=$secret
    $launchInfo.Arguments = '"' + $cliScript + '" --new-window --user-data-dir "' + $ideUserData + '" --extensions-dir "' + $extensionSilo + '" "' + $workspace + '"'
    $cliProcess = [Diagnostics.Process]::Start($launchInfo); $cliProcess.WaitForExit(10000) | Out-Null; Start-Sleep -Seconds 8
    $probe = @(Probe-Processes); $probePids = @($probe | Select-Object -ExpandProperty ProcessId)
    $rootCandidate = @($probe | Where-Object {$protectedPids -notcontains $_.ParentProcessId} | Sort-Object CreationDate | Select-Object -First 1)
    if ($probe.Count -eq 0 -or $rootCandidate.Count -ne 1) { throw 'ANTIGRAVITY_SINGLE_INSTANCE_REUSE' }
    $probeRootPid = [int]$rootCandidate[0].ProcessId
    if (-not (Originals-Alive)) { throw 'ORIGINAL_IDE_PROCESS_LOST' }
    $smokePass = $true
    Add-Event 'PROBE_IDE_STARTED' @{probe_root_pid=$probeRootPid;process_count=$probePids.Count}
    Add-Event 'PARALLEL_SMOKE_PASS' @{different_process_tree=$true;original_instance_alive=$true;temporary_user_data_populated=((Get-ChildItem -LiteralPath $ideUserData -Force | Measure-Object).Count -gt 0)}
    [IO.File]::WriteAllText($baselinePromptPath, "Phase 0B baseline synthetic transport check. Reply briefly.`r`n$baselinePrompt`r`n", $utf8)
    $control = Wait-Control @('baseline-sent') 'WAIT_BASELINE_REQUEST'
    $userBaseline = $control -eq 'baseline-sent'
    Add-Event 'BASELINE_USER_ACTION' @{observation_source='user-assisted';request_ordinal=1}
    $capture1 = Wait-Capture 1
    if (-not (Capture-Pass $capture1 1 $initialModel 'baseline')) { $classification='BASELINE_ROUTE_OR_AUTH_MISMATCH'; throw 'BASELINE_ACCEPTANCE_FAILED' }
    Add-Event 'BASELINE_REQUEST_CAPTURED' @{request_ordinal=1;model_initial=$true;route_responses=$true;provider_auth=$true;sentinel_ingress=$true}
    [IO.File]::WriteAllText($astraPromptPath, "Phase 0B Astra synthetic transport check. Reply briefly.`r`n$astraPrompt`r`n", $utf8)
    $control = Wait-Control @('astra-selected','astra-not-visible') 'WAIT_ASTRA_SELECTION'
    if ($control -eq 'astra-not-visible') { $classification='ASTRA_NOT_VISIBLE'; Add-Event 'ASTRA_VISIBLE' @{observation_source='user-assisted';astra_visible=$false}; throw 'ASTRA_NOT_VISIBLE' }
    $userAstra = $true
    Add-Event 'ASTRA_VISIBLE' @{observation_source='user-assisted';astra_visible=$true}
    Add-Event 'ASTRA_SELECTED' @{observation_source='user-assisted';astra_selected=$true}
    $control = Wait-Control @('astra-sent') 'WAIT_ASTRA_REQUEST'
    Add-Event 'ASTRA_USER_ACTION' @{observation_source='user-assisted';request_ordinal=2}
    $capture2 = Wait-Capture 2
    if (-not (Capture-Pass $capture2 2 'gpt-6-astra' 'astra')) { $classification='CODEX_PICKER_ROUTE_MISMATCH'; throw 'ASTRA_ROUTE_OR_AUTH_MISMATCH' }
    Add-Event 'ASTRA_REQUEST_CAPTURED' @{request_ordinal=2;model_astra=$true;route_responses=$true;same_provider_auth=$true;sentinel_ingress=$true}
    $null = Wait-Control @('probe-closed') 'WAIT_PROBE_CLOSE'
    $status='PASS'; $classification='PASS'
} catch {
    if ($classification -eq 'RUNNING') { $classification=$_.Exception.Message }
    $failure=@{classification=$classification;exception_type=$_.Exception.GetType().FullName;stage=$stage}
    if ($classification -in @('ASTRA_NOT_VISIBLE','BASELINE_ROUTE_OR_AUTH_MISMATCH','CODEX_PICKER_ROUTE_MISMATCH','BASELINE_ACCEPTANCE_FAILED','ASTRA_ROUTE_OR_AUTH_MISMATCH','USER_ASSISTED_PROBE_TIMEOUT')) {
        try { $null=Wait-Control @('probe-closed') 'WAIT_PROBE_CLOSE' } catch { $failure=@{classification=$classification;exception_type=$_.Exception.GetType().FullName;stage='WAIT_PROBE_CLOSE'} }
    }
} finally {
    if ($recorder -and -not $recorder.HasExited) { try { $recorder.Kill(); $recorder.WaitForExit(5000) | Out-Null } catch { $failure=@{classification='RECORDER_CLEANUP_FAILED';exception_type=$_.Exception.GetType().FullName;stage='CLEANUP'}; $status='BLOCKED' } }
    $remainingProbe=@(Probe-Processes); $listenerAfter=if($null -eq $port){0}else{@(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue).Count}
    $realConfigAfter=Safe-Hash $realConfig; $realSettingsAfter=Safe-Hash $realSettings; $extensionManifestFinal=if($sourceExtension.Count -eq 1){Safe-Hash (Join-Path $sourceExtension[0].FullName 'package.json')}else{'UNKNOWN'}
    $cleanup=@{probe_process_absent=($remainingProbe.Count -eq 0);recorder_listener_absent=($listenerAfter -eq 0);original_instance_survived=(Originals-Alive);temporary_environment_not_persisted=(-not (Test-Path Env:DUALPOOL_CODEX_KEY));temporary_root_removed=$false}
    if ($remainingProbe.Count -eq 0) { try { [IO.Directory]::Delete($resolvedSession,$true); $cleanup.temporary_root_removed=(-not [IO.Directory]::Exists($resolvedSession)) } catch { $failure=@{classification='TEMP_CLEANUP_FAILED';exception_type=$_.Exception.GetType().FullName;stage='CLEANUP'}; $status='BLOCKED' } }
    if (@($cleanup.Values | Where-Object {$_ -ne $true}).Count -eq 0) { try { Add-Event 'CLEANUP_PASS' $cleanup } catch {} }
    $configUnchanged=$realConfigBefore -eq $realConfigAfter; $settingsUnchanged=$realSettingsBefore -eq $realSettingsAfter; $extensionUnchanged=$extensionManifestBefore -eq $extensionManifestFinal
    $u005=if($capture1 -and (Capture-Pass $capture1 1 $initialModel 'baseline')){'PROBED'}else{'PARTIAL / UNKNOWN'}
    $u006=if($capture2 -and (Capture-Pass $capture2 2 'gpt-6-astra' 'astra')){'PROBED'}elseif($classification -eq 'CODEX_PICKER_ROUTE_MISMATCH'){'FAIL'}else{'BLOCKED'}
    $result=[ordered]@{schema_version='2';test_id=$testId;run_id=$runId;status=$status;classification=$classification;started_at=$started.ToString('o');ended_at=[DateTime]::UtcNow.ToString('o');environment=@{antigravity_version='1.107.0';extension_id='openai.chatgpt';extension_version=if($extensionPackage){$extensionPackage.version}else{$null};codex_cli_version='0.154.0';initial_model=$initialModel;astra_model='gpt-6-astra'};preflight=@{flags=$flags;parallel_smoke_pass=$smokePass;extension_isolation_pass=$extensionIsolationPass;protected_process_count=$protectedPids.Count;extension_manifest_hash_match=($extensionManifestBefore -eq $extensionManifestAfter)};isolation=@{probe_root_pid=$probeRootPid;different_process_tree=$smokePass;original_instance_remained_alive=(Originals-Alive);temporary_user_data=$true;temporary_codex_home=$true;temporary_extension_silo=$true};user_observation=@{source='user-assisted';baseline_sent=$userBaseline;astra_selected=$userAstra};baseline_request=$capture1;astra_request=$capture2;controls=@{real_config_sha256_before=$realConfigBefore;real_config_sha256_after=$realConfigAfter;real_config_unchanged=$configUnchanged;real_settings_sha256_before=$realSettingsBefore;real_settings_sha256_after=$realSettingsAfter;real_settings_unchanged=$settingsUnchanged;installed_extension_manifest_sha256_before=$extensionManifestBefore;installed_extension_manifest_sha256_after=$extensionManifestFinal;installed_extension_unchanged=$extensionUnchanged};cleanup=$cleanup;failure=$failure;unknowns=@{'U-001'='BLOCKED';'U-002'='BLOCKED';'U-003'='BLOCKED';'U-004'='BLOCKED';'U-005'=$u005;'U-006'=$u006;'U-007'='PROBED';'U-008'='PARTIAL / UNKNOWN'}}
    New-Item -ItemType Directory -Path $EvidenceDirectory -Force | Out-Null
    Write-SafeJson (Join-Path $EvidenceDirectory ('codex-extension-two-request-' + $runId + '.json')) $result
    Write-Output ('RESULT run_id={0} status={1} classification={2}' -f $runId,$status,$classification)
}
if ($status -ne 'PASS') { exit 1 }
