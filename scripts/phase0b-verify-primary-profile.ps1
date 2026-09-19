[CmdletBinding()]
param([switch]$Generate)
$ErrorActionPreference = 'Stop'
if ($Generate) { Write-Warning 'Historical gate generation disabled; running read-only audit.' }
& node (Join-Path $PSScriptRoot 'phase0b-primary-evidence.cjs')
exit $LASTEXITCODE
