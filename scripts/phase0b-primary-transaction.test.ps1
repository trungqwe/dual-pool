$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'phase0b-primary-transaction.ps1')
$root = Join-Path ([IO.Path]::GetTempPath()) ('dual-pool-transaction-test-' + [guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($root) | Out-Null
$count = 0
function Assert($Condition) { if (-not $Condition) { throw 'ASSERTION_FAILED' }; $script:count++ }
function ExpectFailure([scriptblock]$Action, [string]$Code) {
    $failed = $false
    try { & $Action | Out-Null } catch { $failed = $_.Exception.Message -eq $Code }
    Assert $failed
}
try {
    foreach ($present in @($true, $false)) {
        $case = Join-Path $root ([guid]::NewGuid().ToString('N'))
        [IO.Directory]::CreateDirectory($case) | Out-Null
        $config = Join-Path $case 'config.toml'
        if ($present) { [IO.File]::WriteAllText($config, "# untouched comment`nmodel = `"fixture`"`n") }
        $before = Get-ProbeHash $config
        Invoke-ProbeTransaction Prepare $case $config 0 | Out-Null
        Invoke-ProbeTransaction Activate $case $config 12345 | Out-Null
        Assert ((Get-ProbeHash $config) -ne $before)
        $markerPath = Join-Path $case 'transaction.json'
        $marker = Get-Content -Raw $markerPath | ConvertFrom-Json
        # Simulate crash after swap, before ACTIVE write.
        $marker.state = 'ACTIVATING'; Write-ProbeMarker $case $marker
        Invoke-ProbeTransaction Restore $case $config 0 | Out-Null
        Assert ((Get-ProbeHash $config) -eq $before)
        Invoke-ProbeTransaction Restore $case $config 0 | Out-Null
        Assert ((Get-ProbeHash $config) -eq $before)
    }
    foreach ($fault in @('before','after','backup')) {
        $case = Join-Path $root ([guid]::NewGuid().ToString('N'))
        [IO.Directory]::CreateDirectory($case) | Out-Null
        $config = Join-Path $case 'config.toml'
        [IO.File]::WriteAllText($config, 'fixture-original')
        Invoke-ProbeTransaction Prepare $case $config 0 | Out-Null
        if ($fault -eq 'before') {
            [IO.File]::WriteAllText($config, 'external-change')
            ExpectFailure { Invoke-ProbeTransaction Activate $case $config 12345 } 'CONFIG_CONCURRENT_EDIT'
            Assert ([IO.File]::ReadAllText($config) -eq 'external-change')
        } else {
            Invoke-ProbeTransaction Activate $case $config 12345 | Out-Null
            if ($fault -eq 'after') {
                [IO.File]::WriteAllText($config, 'external-change')
                ExpectFailure { Invoke-ProbeTransaction Restore $case $config 0 } 'CONFIG_CONCURRENT_EDIT'
                Assert ([IO.File]::ReadAllText($config) -eq 'external-change')
            } else {
                $active = Get-ProbeHash $config
                [IO.File]::WriteAllText((Join-Path $case 'config.backup'), 'corrupt')
                ExpectFailure { Invoke-ProbeTransaction Restore $case $config 0 } 'RECOVERY_BACKUP_HASH_MISMATCH'
                Assert ((Get-ProbeHash $config) -eq $active)
            }
        }
    }
    @{status='PASS';assertions=$count;real_config_touched=$false} | ConvertTo-Json -Compress
} finally {
    $resolved = [IO.Path]::GetFullPath($root)
    if ((Split-Path $resolved -Parent) -ne ([IO.Path]::GetTempPath()).TrimEnd('\') -or
        (Split-Path $resolved -Leaf) -notmatch '^dual-pool-transaction-test-[a-f0-9]{32}$') { throw 'UNSAFE_TEST_CLEANUP' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
