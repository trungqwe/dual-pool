[CmdletBinding()]
param([switch]$Generate)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
Set-Location $root
$folder = 'evidence/phase-0b-v5'
$securityPath = "$folder/security-gate-v5.json"
$manifestPath = "$folder/phase-0b-v5-evidence-manifest.json"
$started = [DateTime]::UtcNow
$utf8 = New-Object Text.UTF8Encoding($false)
$patterns = [ordered]@{
    sentinel = '(?:SECRET_SENTINEL_|PROMPT_SENTINEL_)[a-fA-F0-9]{32}'
    authorization_cookie_bearer = '(?i)(?:Bearer\s+[a-z0-9._-]{16,}|"(?:authorization|cookie)"\s*:\s*"[^"\r\n]+")'
    token = '(?i)(?:sk-[a-z0-9_-]{20,}|"(?:refresh_token|access_token|client_secret)"\s*:\s*"[a-z0-9._/-]{8,}")'
    private_key = '-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'
    email = '(?i)\b[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}\b'
    literal_user_path = '(?i)C:\\Users\\[a-z0-9_.-]+\\'
    raw_body_fields = '"(?:raw_body|body_text|request_body|response_body|raw_headers|raw_stdout|raw_stderr)"\s*:'
}
# Real scanner positive controls, generated only in memory, never persisted.
$positive = @{
    sentinel = 'SECRET_SENTINEL_' + ('a' * 32)
    authorization_cookie_bearer = 'Bearer ' + ('a' * 24)
    token = 'sk-' + ('a' * 24)
    private_key = '-----BEGIN ' + 'PRIVATE KEY-----'
    email = 'synthetic' + '@' + 'example.invalid'
    literal_user_path = 'C:' + '\Users\fixture\file'
    raw_body_fields = '"raw_' + 'body":'
}
foreach ($key in $patterns.Keys) { if ($positive[$key] -notmatch $patterns[$key]) { throw ('SCANNER_SELFTEST_FAIL:' + $key) } }
function Scan-Files($Paths) {
    $counts = [ordered]@{}
    foreach ($category in $patterns.Keys) { $counts[$category] = 0 }
    foreach ($file in $Paths) {
        $text = [IO.File]::ReadAllText((Join-Path $root $file))
        foreach ($category in $patterns.Keys) { $counts[$category] += [regex]::Matches($text,$patterns[$category]).Count }
    }
    return $counts
}
$files = @(@(git diff --name-only HEAD) + @(git ls-files --others --exclude-standard) | Sort-Object -Unique)
if (-not $Generate) {
    $oldSecurity = Get-Content -Raw $securityPath | ConvertFrom-Json
    $files = @($oldSecurity.paths_actually_scanned)
}
$files = @($files | Where-Object { $_ -ne $securityPath -and $_ -ne $manifestPath -and (Test-Path -LiteralPath $_ -PathType Leaf) })
if ($files.Count -eq 0) { throw 'NO_SCAN_INPUTS' }
$jsonFiles = @(@(git ls-files '*.json') + @($files | Where-Object {$_ -like '*.json'}) | Sort-Object -Unique)
foreach ($file in $jsonFiles) { Get-Content -Raw -Encoding UTF8 $file | ConvertFrom-Json | Out-Null }
$linkCount = 0
foreach ($file in @($files | Where-Object {$_ -like '*.md'})) {
    $content = Get-Content -Raw -Encoding UTF8 $file
    foreach ($match in [regex]::Matches($content,'\[[^\]]+\]\(([^)]+)\)')) {
        $link = $match.Groups[1].Value.Trim('<','>')
        if ($link -match '^https?://|^#') { continue }
        $link = ($link -split '#')[0]
        $target = Join-Path (Split-Path (Join-Path $root $file)) $link
        if (-not (Test-Path -LiteralPath $target)) { throw ('BROKEN_DOC_LINK:' + $file) }
        $linkCount++
    }
}
$parseTokens = $null; $parseErrors = $null
foreach ($file in @($files | Where-Object {$_ -like '*.ps1'})) {
    [Management.Automation.Language.Parser]::ParseFile((Join-Path $root $file),[ref]$parseTokens,[ref]$parseErrors) | Out-Null
    if ($parseErrors.Count) { throw ('POWERSHELL_PARSE_FAIL:' + $file) }
}
& node --check scripts/phase0b-recorder-v5.cjs
if ($LASTEXITCODE -ne 0) { throw 'NODE_PARSE_FAIL' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'DIFF_CHECK_FAIL' }
$statusRecord = Get-Content -Raw "$folder/phase-0b-status-v5.json" | ConvertFrom-Json
$index = Get-Content -Raw "$folder/probe-results-v5.json" | ConvertFrom-Json
$probe = Get-Content -Raw (Join-Path $folder $index.authoritative_result) | ConvertFrom-Json
if ($probe.status -ne 'PASS' -or @($probe.assertions.PSObject.Properties | Where-Object {$_.Value -ne $true}).Count -ne 0) { throw 'GATE_A_NOT_PASS' }
if ($statusRecord.gate_a -ne $probe.status) { throw 'STATUS_MISMATCH' }
$legacy = @(Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'powershell.exe' -and $_.CommandLine -match 'dual-pool-phase0b-v4-[a-f0-9]{32}[\\/]recorder\.ps1' })
$live = @(Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'node.exe' -and $_.CommandLine -match 'phase0b-recorder-v5\.cjs' })
$cleanup = @{
    legacy_v4_recorders_after = $legacy.Count
    v5_recorders_after = $live.Count
    v5_temp_roots = @(Get-ChildItem -LiteralPath $env:TEMP -Directory -Filter 'dual-pool-phase0b-v5-*').Count
    old_diagnostic_absent = -not(Test-Path (Join-Path $env:TEMP 'dual-pool-phase0b-v4-diagnostic.txt'))
    key_environment_absent = -not(Test-Path Env:DUALPOOL_CODEX_KEY)
    listener_after = @(Get-NetTCPConnection -State Listen -LocalPort $probe.listener.observations[0].port -ErrorAction SilentlyContinue).Count
}
if ($cleanup.legacy_v4_recorders_after -ne 0 -or $cleanup.v5_recorders_after -ne 0 -or $cleanup.v5_temp_roots -ne 0 -or $cleanup.listener_after -ne 0 -or -not $cleanup.old_diagnostic_absent -or -not $cleanup.key_environment_absent) { throw 'CLEANUP_FAIL' }
$counts = Scan-Files $files
if (@($counts.Values | Where-Object {$_ -ne 0}).Count -gt 0) { throw 'SECRET_LEAK_DETECTED' }
if ($Generate) {
    if (Test-Path $manifestPath) { throw 'IMMUTABLE_MANIFEST_EXISTS' }
    $security = @{schema_version='5';test_id='P0B-SECURITY-005';status='PASS';started_at=$started.ToString('o');ended_at=[DateTime]::UtcNow.ToString('o');procedure='scripts/phase0b-verify-v5.ps1';scanner_selftest='PASS';paths_actually_scanned=$files;pattern_classes=@($patterns.Keys);matches_by_category=$counts;match_count=0;json_validation='PASS';json_files_checked=$jsonFiles.Count;documentation_links='PASS';local_links_checked=$linkCount;diff_check='PASS';cleanup=$cleanup;probe_ended_at=$probe.ended_at}
    [IO.File]::WriteAllText((Join-Path $root $securityPath),($security | ConvertTo-Json -Depth 12)+"`n",$utf8)
    $artifactFiles = @(Get-ChildItem -LiteralPath $folder -File -Filter '*.json' | Where-Object {$_.Name -ne 'phase-0b-v5-evidence-manifest.json'} | Sort-Object Name)
    $artifacts = @($artifactFiles | ForEach-Object { @{path=$_.Name;sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash} })
    $sourceFiles = @('scripts/phase0b-codex-cli-transport-v5.ps1','scripts/phase0b-recorder-v5.cjs','scripts/phase0b-verify-v5.ps1')
    $sourceHashes = @($sourceFiles | ForEach-Object { @{path=$_;sha256=(Get-FileHash -LiteralPath $_ -Algorithm SHA256).Hash} })
    $manifest = @{schema_version='5';test_id='P0B-MANIFEST-005';status='PASS';gate_a=$probe.status;secret_scan='PASS';json_validation='PASS';cleanup='PASS';generated_at=[DateTime]::UtcNow.ToString('o');artifacts=$artifacts;sources=$sourceHashes;authoritative_status='phase-0b-status-v5.json'}
    $postCounts = Scan-Files @($securityPath)
    if (@($postCounts.Values | Where-Object {$_ -ne 0}).Count -gt 0) { throw 'SECURITY_OUTPUT_LEAK' }
    [IO.File]::WriteAllText((Join-Path $root $manifestPath),($manifest | ConvertTo-Json -Depth 12)+"`n",$utf8)
}
$manifest = Get-Content -Raw $manifestPath | ConvertFrom-Json
foreach ($artifact in $manifest.artifacts) {
    if ((Get-FileHash -LiteralPath (Join-Path $folder $artifact.path) -Algorithm SHA256).Hash -ne $artifact.sha256) { throw ('ARTIFACT_HASH_MISMATCH:' + $artifact.path) }
}
foreach ($source in $manifest.sources) {
    if ((Get-FileHash -LiteralPath $source.path -Algorithm SHA256).Hash -ne $source.sha256) { throw ('SOURCE_HASH_MISMATCH:' + $source.path) }
}
$finalCounts = Scan-Files @($securityPath,$manifestPath)
if (@($finalCounts.Values | Where-Object {$_ -ne 0}).Count -gt 0) { throw 'FINAL_EVIDENCE_LEAK' }
Write-Output ('VERIFY PASS: files={0}; JSON={1}; links={2}; matches=0; hashes=PASS; cleanup=PASS' -f $files.Count,$jsonFiles.Count,$linkCount)
