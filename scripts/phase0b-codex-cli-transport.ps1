$ErrorActionPreference = 'Stop'

$runId = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')
$root = Join-Path ([IO.Path]::GetTempPath()) ('dual-pool-phase0b-' + [Guid]::NewGuid().ToString('N'))
$codexHome = Join-Path $root 'codex-home'
$recorder = Join-Path $root 'recorder.ps1'
$capture = Join-Path $root 'capture.json'
$stdout = Join-Path $root 'codex.stdout.log'
$stderr = Join-Path $root 'codex.stderr.log'
$port = 18000 + (Get-Random -Minimum 100 -Maximum 900)
$key = 'phase0b-sentinel-' + [Guid]::NewGuid().ToString('N')
$started = [DateTime]::UtcNow
$codex = $null
$listener = $null
$ownedPids = @()
$result = [ordered]@{
  schema_version = '1.0'
  test_id = 'phase0b-codex-cli-transport-v2'
  run_id = $runId
  started_at_utc = $started.ToString('o')
  environment = [ordered]@{ os = [Environment]::OSVersion.VersionString; architecture = [Environment]::Is64BitOperatingSystem }
  command_shape = 'codex exec --json --skip-git-repo-check -m gpt-6-astra -c model_provider=dualpool_codex -c model_providers.dualpool_codex.* <synthetic prompt>'
  assertions = @()
  status = 'FAIL'
  cleanup = [ordered]@{ temp_root_removed = $false; owned_processes_after = -1; listener_after = -1; raw_capture_persisted = $false }
  limitations = @('The synthetic recorder rejects the response after metadata capture; no provider account is used.')
}

try {
  $codexCommand = Get-Command codex -ErrorAction Stop
  $codexPath = $codexCommand.Source
  $codexVersion = (& $codexPath --version 2>&1 | Select-Object -First 1).ToString()
  $codexHash = (Get-FileHash -LiteralPath $codexPath -Algorithm SHA256).Hash
  $result.environment.codex_version = $codexVersion
  $result.environment.codex_sha256 = $codexHash
  New-Item -ItemType Directory -Path $codexHome -Force | Out-Null

  @'
param([int]$Port, [string]$Capture, [string]$Sentinel)
$ErrorActionPreference = 'Stop'
function Get-Shape($v, $p) {
  $out = New-Object System.Collections.Generic.List[string]
  if ($null -eq $v) { $out.Add($p + ':null'); return $out }
  if ($v -is [Collections.IDictionary]) { $out.Add($p + ':object'); foreach($k in $v.Keys){ foreach($x in (Get-Shape $v[$k] ($p+'.'+$k))){$out.Add($x)} }; return $out }
  if ($v -is [Collections.IEnumerable] -and -not ($v -is [string])) { $out.Add($p + ':array'); $i=0; foreach($x in $v){ foreach($y in (Get-Shape $x ($p+'[]'))){$out.Add($y)}; $i++ }; return $out }
  $props = @($v.PSObject.Properties)
  if ($props.Count -gt 0 -and $v -isnot [string] -and $v -isnot [ValueType]) { $out.Add($p + ':object'); foreach($prop in $props){ foreach($x in (Get-Shape $prop.Value ($p+'.'+$prop.Name))){$out.Add($x)} }; return $out }
  $out.Add($p + ':' + $v.GetType().Name.ToLowerInvariant()); return $out
}
$l = [Net.HttpListener]::new(); $l.Prefixes.Add(('http://127.0.0.1:{0}/' -f $Port)); $l.Start()
try {
  while ($true) {
    $c = $l.GetContext(); $req = $c.Request
    if ($req.HttpMethod -eq 'GET' -and $req.Url.AbsolutePath -eq '/v1/models') {
      $b = [Text.Encoding]::UTF8.GetBytes('{"data":[{"id":"gpt-6-astra","object":"model"}]}')
      $c.Response.ContentType='application/json'; $c.Response.StatusCode=200; $c.Response.OutputStream.Write($b,0,$b.Length); $c.Response.Close(); continue
    }
    $reader = [IO.StreamReader]::new($req.InputStream); $body = $reader.ReadToEnd(); $reader.Close()
    $json = $null; try { $json = $body | ConvertFrom-Json } catch {}
    $meta = [ordered]@{ schema_version='1.0'; method=$req.HttpMethod; route=$req.Url.AbsolutePath; content_type=$req.ContentType; header_names=@($req.Headers.AllKeys | Sort-Object); authorization_header_present=($null -ne $req.Headers['Authorization']); model=$json.model; request_shape=@(Get-Shape $json 'body'); body_persisted=$false; sentinel_seen_in_persisted_output=$false }
    $meta | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $Capture -Encoding UTF8
    $c.Response.StatusCode=400; $c.Response.Close()
  }
} finally { $l.Stop(); $l.Close() }
'@ | Set-Content -LiteralPath $recorder -Encoding UTF8
  $rp = Start-Process powershell.exe -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-File',$recorder,'-Port',$port,'-Capture',$capture,'-Sentinel',$key) -PassThru -WindowStyle Hidden
  $ownedPids += $rp.Id
  Start-Sleep -Milliseconds 500
  $env:CODEX_HOME = $codexHome
  $args = @('exec','--json','--skip-git-repo-check','-m','gpt-6-astra','-c','model_provider="dualpool_codex"','-c','model_providers.dualpool_codex.name="DualPoolPhase0B"','-c',('model_providers.dualpool_codex.base_url="http://127.0.0.1:{0}/v1"' -f $port),'-c','model_providers.dualpool_codex.wire_api="responses"','-c','model_providers.dualpool_codex.env_key="DUALPOOL_CODEX_KEY"','PHASE0B_SYNTHETIC_ONLY')
  $env:DUALPOOL_CODEX_KEY = $key
  $codex = Start-Process -FilePath $codexPath -ArgumentList $args -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru -WindowStyle Hidden
  $ownedPids += $codex.Id
  if (-not $codex.WaitForExit(15000)) { $codex.Kill(); $codex.WaitForExit() }
  if (Test-Path $capture) {
    $m = Get-Content -Raw $capture | ConvertFrom-Json
    $result.observed = $m
    if ($m.route -eq '/v1/responses' -and $m.model -eq 'gpt-6-astra' -and $m.body_persisted -eq $false -and $m.authorization_header_present) { $result.assertions += 'responses_route_model_auth_metadata_only' }
  }
  $codex.Refresh(); $result.process_exit_code = $codex.ExitCode
  if ($result.assertions.Count -eq 1) { $result.status = 'PASS' }
} finally {
  if ($codex -and -not $codex.HasExited) { $codex.Kill() }
  if ($rp -and -not $rp.HasExited) { $rp.Kill() }
  Remove-Item Env:CODEX_HOME -ErrorAction SilentlyContinue; Remove-Item Env:DUALPOOL_CODEX_KEY -ErrorAction SilentlyContinue
  $result.cleanup.owned_processes_after = @($ownedPids | Where-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue }).Count
  $result.cleanup.listener_after = @(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue).Count
  $capture_existed = Test-Path $capture
  $result.ended_at_utc = [DateTime]::UtcNow.ToString('o')
  $result.duration_ms = ([DateTime]::UtcNow - $started).TotalMilliseconds
  Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 200
  if (Test-Path $root) { Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue }
  $result.cleanup.temp_root_removed = -not (Test-Path $root)
  $result.cleanup.raw_capture_persisted = Test-Path $root
  $json = $result | ConvertTo-Json -Depth 30
  $artifact = Join-Path (Get-Location) ('evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/codex-cli-transport-v2-' + $runId + '.json')
  $json | Set-Content -LiteralPath $artifact -Encoding UTF8
}
if ($result.status -ne 'PASS' -or -not $result.cleanup.temp_root_removed -or $result.cleanup.owned_processes_after -ne 0 -or $result.cleanup.listener_after -ne 0 -or $result.cleanup.raw_capture_persisted) { exit 1 }
