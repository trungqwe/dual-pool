$ErrorActionPreference = 'Stop'
$wrapper = Join-Path $PSScriptRoot 'phase0b-u006-primary-watchdog.ps1'
$session = Join-Path ([IO.Path]::GetTempPath()) ('dual-pool-disabled-test-' + [guid]::NewGuid().ToString('N'))
$config = Join-Path $env:USERPROFILE '.codex/config.toml'
$before = (Get-FileHash -LiteralPath $config -Algorithm SHA256).Hash
$savedPreference = $ErrorActionPreference
$ErrorActionPreference = 'Continue'
$output = & powershell.exe -NoProfile -NonInteractive -File $wrapper -SessionRoot $session 2>&1
$ErrorActionPreference = $savedPreference
$exit = $LASTEXITCODE
$after = (Get-FileHash -LiteralPath $config -Algorithm SHA256).Hash
if ($exit -eq 0) { throw 'DISABLED_ENTRYPOINT_DID_NOT_FAIL' }
if (-not (($output | Out-String) -match 'PRIMARY_REAL_CONFIG_MUTATION_DISABLED')) { throw 'DISABLED_ERROR_MISSING' }
if ($before -ne $after) { throw 'REAL_CONFIG_CHANGED' }
if (Test-Path -LiteralPath $session) { throw 'SESSION_CREATED' }
Write-Output 'PASS: disabled entrypoint created no session and preserved config hash'
