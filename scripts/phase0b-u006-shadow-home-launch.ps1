[CmdletBinding()]
param()
$root = Split-Path -Parent $PSScriptRoot
$watchdog = Join-Path $PSScriptRoot 'phase0b-u006-shadow-home-watchdog.cjs'
if (-not (Test-Path -LiteralPath $watchdog -PathType Leaf)) { throw 'SHADOW_WATCHDOG_NOT_FOUND' }
$node = (Get-Command node.exe -ErrorAction Stop).Source
Start-Process -FilePath powershell.exe -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-Command',"& '$node' '$watchdog'") -WorkingDirectory $root -WindowStyle Normal -Wait
