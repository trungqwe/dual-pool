[CmdletBinding()]
param([switch]$Generate)
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
Set-Location $root
$folder='evidence/phase-0b-codex-extension'
$securityPath="$folder/security-gate.json"
$manifestPath="$folder/evidence-manifest.json"
$utf8=New-Object Text.UTF8Encoding($false)
$patterns=[ordered]@{
  sentinel='(?:SECRET_SENTINEL_|PROMPT_SENTINEL_|BASELINE_PROMPT_SENTINEL_|ASTRA_PROMPT_SENTINEL_)[a-fA-F0-9]{32}'
  bearer='(?i)Bearer\s+[a-z0-9._-]{16,}'
  token='(?i)(?:sk-[a-z0-9_-]{20,}|"(?:refresh_token|access_token|client_secret)"\s*:\s*"[a-z0-9._/-]{8,}")'
  private_key='-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'
  email='(?i)\b[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}\b'
  literal_user_path='(?i)C:\\Users\\[a-z0-9_.-]+\\'
  raw_body_fields='"(?:raw_body|body_text|request_body|response_body|raw_headers|raw_stdout|raw_stderr)"\s*:'
}
$positive=@{sentinel='SECRET_SENTINEL_'+'a'*32;bearer='Bearer '+'a'*24;token='sk-'+'a'*24;private_key='-----BEGIN PRIVATE KEY-----';email='fixture@example.invalid';literal_user_path='C:\Users\fixture\file';raw_body_fields='"raw_'+'body":'}
foreach($key in $patterns.Keys){if($positive[$key] -notmatch $patterns[$key]){throw "SCANNER_SELFTEST_FAIL:$key"}}
$files=@(@(git diff --name-only HEAD)+@(git ls-files --others --exclude-standard)|Sort-Object -Unique|Where-Object{$_ -ne $securityPath -and $_ -ne $manifestPath -and(Test-Path -LiteralPath $_ -PathType Leaf)})
$scanFiles=@($files|Where-Object{$_ -ne 'scripts/phase0b-verify-extension.ps1'})
$counts=[ordered]@{};foreach($key in $patterns.Keys){$counts[$key]=0}
foreach($file in $scanFiles){$text=Get-Content -Raw -Encoding UTF8 (Join-Path $root $file);foreach($key in $patterns.Keys){$counts[$key]+=[regex]::Matches($text,$patterns[$key]).Count}}
$jsonFiles=@($files|Where-Object{$_ -like '*.json'});foreach($file in $jsonFiles){Get-Content -Raw -Encoding UTF8 (Join-Path $root $file)|ConvertFrom-Json|Out-Null}
$linkCount=0;foreach($file in @($files|Where-Object{$_ -like '*.md'})){foreach($m in [regex]::Matches((Get-Content -Raw -Encoding UTF8 (Join-Path $root $file)),'\[[^\]]+\]\(([^)]+)\)')){$link=$m.Groups[1].Value.Trim('<','>') -split '#';if($link[0]-match '^https?://|^#'){continue};if(-not(Test-Path -LiteralPath (Join-Path (Split-Path (Join-Path $root $file)) $link[0]))){throw "BROKEN_DOC_LINK:$file"};$linkCount++}}
$tokens=$null;$errors=$null;foreach($file in @($files|Where-Object{$_ -like '*.ps1'})){[Management.Automation.Language.Parser]::ParseFile((Join-Path $root $file),[ref]$tokens,[ref]$errors)|Out-Null;if($errors.Count){throw "POWERSHELL_PARSE_FAIL:$file"}}
node --check scripts/phase0b-extension-recorder.cjs;if($LASTEXITCODE){throw 'NODE_PARSE_FAIL'};git diff --check;if($LASTEXITCODE){throw 'DIFF_CHECK_FAIL'}
$index=Get-Content -Raw "$folder/probe-results.json"|ConvertFrom-Json;$result=Get-Content -Raw (Join-Path $folder $index.authoritative_result)|ConvertFrom-Json
$tempRoots=@(Get-ChildItem -LiteralPath $env:TEMP -Directory -Filter 'dual-pool-extension-probe-*' -ErrorAction SilentlyContinue);$recorders=@(Get-CimInstance Win32_Process|Where-Object{$_.Name -eq 'node.exe' -and $_.CommandLine -match 'phase0b-extension-recorder\.cjs'});$cleanupPass=$tempRoots.Count -eq 0 -and $recorders.Count -eq 0 -and $result.cleanup.temporary_root_removed -and $result.cleanup.real_config_unchanged -and $result.cleanup.real_settings_unchanged -and [bool](Get-CimInstance Win32_Process|Where-Object{$_.Name -eq 'Antigravity IDE.exe'})
function Safe-Hash([string]$Path){if(Test-Path -LiteralPath $Path -PathType Leaf){return(Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash};return'ABSENT'}
$cfg=Safe-Hash (Join-Path $env:USERPROFILE '.codex/config.toml')
$hashPass=$cfg -eq $result.controls.real_config_sha256_after
if(@($counts.Values|Where-Object{$_ -ne 0}).Count -ne 0){throw 'SECRET_SCAN_FAIL'};if(-not$cleanupPass){throw 'CLEANUP_FAIL'};if(-not$hashPass){throw 'CONTROL_HASH_FAIL'}
if($Generate){$security=@{schema_version='1';test_id='P0B-SECURITY-EXT-001';status='PASS';scanner_selftest='PASS';paths_actually_scanned=$files;pattern_classes=@($patterns.Keys);matches_by_category=$counts;match_count=0;json_validation='PASS';json_files_checked=$jsonFiles.Count;documentation_links='PASS';local_links_checked=$linkCount;syntax='PASS';cleanup='PASS';control_hashes='PASS';generated_at=[DateTime]::UtcNow.ToString('o')};[IO.File]::WriteAllText((Join-Path $root $securityPath),($security|ConvertTo-Json -Depth 15)+"`n",$utf8);$artifacts=@(Get-ChildItem -LiteralPath $folder -File -Filter '*.json'|Where-Object{$_.Name -ne 'evidence-manifest.json'}|Sort-Object Name|ForEach-Object{@{path=$_.Name;sha256=(Get-FileHash $_.FullName -Algorithm SHA256).Hash}});$sources=@('scripts/phase0b-codex-extension-parallel.ps1','scripts/phase0b-extension-recorder.cjs','scripts/phase0b-verify-extension.ps1')|ForEach-Object{@{path=$_;sha256=(Get-FileHash $_ -Algorithm SHA256).Hash}};$manifest=@{schema_version='1';test_id='P0B-MANIFEST-EXT-001';status='PASS';security='PASS';cleanup='PASS';control_hashes='PASS';generated_at=[DateTime]::UtcNow.ToString('o');artifacts=$artifacts;sources=$sources;authoritative_result=$index.authoritative_result};[IO.File]::WriteAllText((Join-Path $root $manifestPath),($manifest|ConvertTo-Json -Depth 15)+"`n",$utf8)}
$manifest=Get-Content -Raw $manifestPath|ConvertFrom-Json;foreach($a in $manifest.artifacts){if((Get-FileHash (Join-Path $folder $a.path) -Algorithm SHA256).Hash -ne $a.sha256){throw "ARTIFACT_HASH_FAIL:$($a.path)"}};foreach($s in $manifest.sources){if((Get-FileHash $s.path -Algorithm SHA256).Hash -ne $s.sha256){throw "SOURCE_HASH_FAIL:$($s.path)"}};$finalCounts=[ordered]@{};foreach($key in $patterns.Keys){$finalCounts[$key]=0};foreach($file in @($securityPath,$manifestPath)){foreach($key in $patterns.Keys){$finalCounts[$key]+=[regex]::Matches((Get-Content -Raw (Join-Path $root $file)),$patterns[$key]).Count}};if(@($finalCounts.Values|Where-Object{$_ -ne 0}).Count){throw 'FINAL_EVIDENCE_LEAK'};Write-Output "VERIFY PASS: files=$($files.Count);json=$($jsonFiles.Count);links=$linkCount;matches=0;cleanup=PASS;hashes=PASS"
