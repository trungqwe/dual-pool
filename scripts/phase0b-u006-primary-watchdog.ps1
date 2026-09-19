[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$SessionRoot)
$ErrorActionPreference = 'Stop'
& node (Join-Path $PSScriptRoot 'phase0b-primary-watchdog.cjs') $SessionRoot
exit $LASTEXITCODE
