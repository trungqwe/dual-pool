[CmdletBinding()]
param(
    [ValidateSet('Prepare','Activate','Restore')][string]$Action,
    [string]$SessionRoot,
    [int]$Port
)
$ErrorActionPreference = 'Stop'

function Get-ProbeHash([string]$File) {
    if (-not [IO.File]::Exists($File)) { return 'ABSENT' }
    return (Get-FileHash -LiteralPath $File -Algorithm SHA256).Hash
}

function Write-ProbeFile([string]$File, [byte[]]$Bytes) {
    $stream = [IO.File]::Open($File, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
    try { $stream.Write($Bytes, 0, $Bytes.Length); $stream.Flush($true) } finally { $stream.Dispose() }
}

function Set-ProbeAtomic([string]$File, [byte[]]$Bytes, [string]$Expected) {
    if ([IO.File]::Exists($File) -and ((Get-Item -LiteralPath $File -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw 'CONFIG_REPARSE_POINT_REFUSED'
    }
    $stage = $File + '.dualpool-stage-' + [guid]::NewGuid().ToString('N')
    $displaced = $stage + '.previous'
    Write-ProbeFile $stage $Bytes
    $lease = $null
    try {
        if ($Expected -ne 'ABSENT') {
            # Deny in-place writers while checking and replacing. Keep displaced bytes
            # on any failure, including another application's concurrent rename.
            $lease = [IO.File]::Open($File, 'Open', 'Read', ([IO.FileShare]::Read -bor [IO.FileShare]::Delete))
        }
        if ((Get-ProbeHash $File) -ne $Expected) { throw 'CONFIG_CONCURRENT_EDIT' }
        if ($Expected -eq 'ABSENT') {
            [IO.File]::Move($stage, $File)
        } else {
            [IO.File]::Replace($stage, $File, $displaced, $true)
            if ((Get-ProbeHash $displaced) -ne $Expected) { throw 'CONFIG_REPLACE_CONFLICT_BACKUP_RETAINED' }
        }
    } finally {
        if ($lease) { $lease.Dispose() }
        if ([IO.File]::Exists($stage)) { [IO.File]::Delete($stage) }
    }
    # Cleanup cannot turn a completed mutation into an unrecorded activation.
    if ([IO.File]::Exists($displaced)) { try { [IO.File]::Delete($displaced) } catch {} }
}

function Write-ProbeMarker([string]$Root, $Marker) {
    $file = Join-Path $Root 'transaction.json'
    $bytes = [Text.Encoding]::UTF8.GetBytes(($Marker | ConvertTo-Json -Depth 5))
    Set-ProbeAtomic $file $bytes (Get-ProbeHash $file)
}

function Test-ProbeToml([string]$Content) {
    $info = New-Object Diagnostics.ProcessStartInfo
    $info.FileName = (Get-Command python.exe -ErrorAction Stop).Source
    $info.Arguments = '-c "import sys,tomllib; tomllib.loads(sys.stdin.read())"'
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardInput = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $proc = [Diagnostics.Process]::Start($info)
    try {
        $proc.StandardInput.Write($Content); $proc.StandardInput.Close()
        if (-not $proc.WaitForExit(10000)) { $proc.Kill(); throw 'TOML_VALIDATION_TIMEOUT' }
        if ($proc.ExitCode -ne 0) { throw 'CONFIG_PROBE_TOML_INVALID' }
    } finally { $proc.Dispose() }
}

function Invoke-ProbeTransaction([string]$Operation, [string]$Root, [string]$Config, [int]$RecorderPort) {
    $markerFile = Join-Path $Root 'transaction.json'
    $backup = Join-Path $Root 'config.backup'
    $run = Split-Path $Root -Leaf
    $sibling = $Config + '.dualpool-backup-' + $run
    if ($Operation -eq 'Prepare') {
        if ([IO.File]::Exists($markerFile)) { throw 'SESSION_ALREADY_PREPARED' }
        $before = Get-ProbeHash $Config
        if ($before -ne 'ABSENT') {
            [IO.File]::Copy($Config, $backup, $false)
            [IO.File]::Copy($Config, $sibling, $false)
            if ((Get-ProbeHash $backup) -ne $before -or (Get-ProbeHash $sibling) -ne $before) { throw 'CONFIG_BACKUP_FAILED' }
        }
        $marker = @{schema_version=2; owner=$run; before=$before; probe='ABSENT'; state='PREPARED'}
        Write-ProbeMarker $Root $marker
        return @{status='PASS';before_sha256=$before;backup_created=($before -ne 'ABSENT')}
    }
    if (-not [IO.File]::Exists($markerFile)) { throw 'RECOVERY_MARKER_ABSENT' }
    $marker = Get-Content -LiteralPath $markerFile -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($marker.schema_version -ne 2 -or $marker.owner -ne $run) { throw 'RECOVERY_OWNER_MISMATCH' }
    if ($marker.state -eq 'CONFLICT') { throw 'CONFIG_REPLACE_CONFLICT_BACKUP_RETAINED' }
    if ($marker.before -ne 'ABSENT' -and (Get-ProbeHash $backup) -ne $marker.before) { throw 'RECOVERY_BACKUP_HASH_MISMATCH' }
    if ($Operation -eq 'Activate') {
        if ($marker.state -ne 'PREPARED' -or $RecorderPort -lt 1024 -or $RecorderPort -gt 65535) { throw 'INVALID_ACTIVATION' }
        $content = "# Dual Pool temporary U-006 probe`nmodel_provider = `"dualpool_probe`"`n`n[model_providers.dualpool_probe]`nname = `"Dual Pool U-006`"`nbase_url = `"http://127.0.0.1:$RecorderPort/v1`"`nwire_api = `"responses`"`nenv_key = `"DUALPOOL_CODEX_KEY`"`n"
        $bytes = [Text.Encoding]::UTF8.GetBytes($content)
        Test-ProbeToml $content
        $sha = [Security.Cryptography.SHA256]::Create()
        try { $marker.probe = ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','') } finally { $sha.Dispose() }
        # Durable intent precedes replacement; recovery does not depend on a later flag.
        $marker.state = 'ACTIVATING'
        Write-ProbeMarker $Root $marker
        try { Set-ProbeAtomic $Config $bytes $marker.before } catch {
            if ($_.Exception.Message -eq 'CONFIG_REPLACE_CONFLICT_BACKUP_RETAINED') {
                $marker.state = 'CONFLICT'; Write-ProbeMarker $Root $marker
            }
            throw
        }
        if ((Get-ProbeHash $Config) -ne $marker.probe) { throw 'CONFIG_PROBE_WRITE_FAILED' }
        Test-ProbeToml ([IO.File]::ReadAllText($Config))
        $marker.state = 'ACTIVE'
        Write-ProbeMarker $Root $marker
        return @{status='PASS';probe_sha256=$marker.probe}
    }
    $current = Get-ProbeHash $Config
    if ($current -ne $marker.before) {
        if ($current -ne $marker.probe -or $marker.probe -eq 'ABSENT') { throw 'CONFIG_CONCURRENT_EDIT' }
        if ($marker.before -eq 'ABSENT') {
            # Rename the owned probe aside; never recursively delete config directories.
            $removed = Join-Path (Split-Path $Config) ('.dualpool-removed-' + [guid]::NewGuid().ToString('N'))
            [IO.File]::Move($Config, $removed)
            if ((Get-ProbeHash $removed) -ne $marker.probe) { throw 'CONFIG_REMOVE_CONFLICT_BACKUP_RETAINED' }
            [IO.File]::Delete($removed)
        } else { Set-ProbeAtomic $Config ([IO.File]::ReadAllBytes($backup)) $marker.probe }
    }
    if ((Get-ProbeHash $Config) -ne $marker.before) { throw 'CONFIG_RESTORE_MISMATCH' }
    $marker.state = 'RESTORED'
    Write-ProbeMarker $Root $marker
    return @{status='PASS';before_sha256=$marker.before;after_sha256=(Get-ProbeHash $Config);restore_verified=$true}
}

if ($Action) {
    try {
        $root = [IO.Path]::GetFullPath($SessionRoot)
        $parent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')
        if ((Split-Path $root -Parent) -ne $parent -or (Split-Path $root -Leaf) -notmatch '^dual-pool-u006-primary-[a-f0-9]{32}$') { throw 'UNSAFE_SESSION_ROOT' }
        if ((Get-Item -LiteralPath $root -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'UNSAFE_SESSION_ROOT' }
        Invoke-ProbeTransaction $Action $root (Join-Path $env:USERPROFILE '.codex/config.toml') $Port | ConvertTo-Json -Compress
    } catch {
        $code = $_.Exception.Message
        if ($code -notmatch '^[A-Z][A-Z0-9_]+$') { $code = 'TRANSACTION_IO_FAILED' }
        Write-Output ($code | ConvertTo-Json -Compress)
        exit 1
    }
}
