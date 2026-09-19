[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$SessionRoot)
$ErrorActionPreference = 'Stop'
# No process termination. Close the probe voluntarily before recovery.
if (@(Get-Process -Name 'Antigravity IDE' -ErrorAction SilentlyContinue).Count -gt 0) {
    throw 'CLOSE_PRIMARY_BEFORE_RECOVERY'
}
& powershell.exe -NoProfile -NonInteractive -File (Join-Path $PSScriptRoot 'phase0b-primary-transaction.ps1') -Action Restore -SessionRoot $SessionRoot
exit $LASTEXITCODE
