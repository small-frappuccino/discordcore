<#
.SYNOPSIS
Migrates a user-scoped Scoop installation to a new root directory.

.DESCRIPTION
This script is designed for a user-scoped Scoop install like:
  C:\Users\<user>\scoop -> D:\Scoop

It creates a logical backup, copies the Scoop root with robocopy while
preserving links, rewrites user environment variables that still point to the
old root, updates Scoop configuration, runs `scoop reset -a`, and validates the
result with `scoop checkup`.

Run it from PowerShell started outside VS Code and with Scoop-managed apps
closed.

.EXAMPLE
pwsh -ExecutionPolicy Bypass -File .\scripts\scoop-migrate-root.ps1

.EXAMPLE
pwsh -ExecutionPolicy Bypass -File .\scripts\scoop-migrate-root.ps1 -DeleteSourceRootAfterSuccess
#>

[CmdletBinding()]
param(
    [string]$SourceRoot = (Join-Path $HOME 'scoop'),
    [string]$TargetRoot = 'D:\Scoop',
    [string]$BackupRoot,
    [switch]$SkipBackup,
    [switch]$SkipRunningProcessCheck,
    [switch]$DeleteSourceRootAfterSuccess
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Step {
    param([string]$Message)

    Write-Host ''
    Write-Host ('==> ' + $Message) -ForegroundColor Cyan
}

function Resolve-NormalizedPath {
    param([Parameter(Mandatory)] [string]$Path)

    return [System.IO.Path]::GetFullPath($Path)
}

function Test-ScoopArtifacts {
    param([Parameter(Mandatory)] [string]$Root)

    $shimPath = Join-Path $Root 'shims\scoop.ps1'
    $corePath = Join-Path $Root 'apps\scoop\current\bin\scoop.ps1'

    return (Test-Path $shimPath) -and (Test-Path $corePath)
}

function Resolve-ScoopShim {
    param([string[]]$PreferredRoots)

    foreach ($root in $PreferredRoots) {
        if ([string]::IsNullOrWhiteSpace($root)) {
            continue
        }

        $candidate = Join-Path $root 'shims\scoop.ps1'
        if (Test-Path $candidate) {
            return (Resolve-NormalizedPath $candidate)
        }
    }

    $command = Get-Command scoop -ErrorAction SilentlyContinue
    if ($null -ne $command) {
        if ($command.Source -and ($command.Source -like '*.ps1')) {
            return (Resolve-NormalizedPath $command.Source)
        }

        if ($command.Path -and ($command.Path -like '*.ps1')) {
            return (Resolve-NormalizedPath $command.Path)
        }
    }

    throw 'Unable to resolve scoop.ps1. Ensure Scoop is installed before running this script.'
}

function Invoke-ScoopCommand {
    param(
        [Parameter(Mandatory)] [string]$ScoopShim,
        [Parameter(Mandatory)] [string[]]$Arguments
    )

    & $ScoopShim @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw ('Scoop command failed: ' + (($Arguments -join ' ')))
    }
}

function Get-RunningProcessesFromRoot {
    param([Parameter(Mandatory)] [string]$Root)

    $rootPattern = $Root.TrimEnd('\') + '\*'

    return @(Get-Process -ErrorAction SilentlyContinue |
        Where-Object {
            $_.Path -and
            ($_.Path -like $rootPattern)
        } |
        Select-Object @{ Name = 'ProcessId'; Expression = { $_.Id } }, Name, @{ Name = 'ExecutablePath'; Expression = { $_.Path } } |
        Sort-Object ProcessId -Unique)
}

function Ensure-Directory {
    param([Parameter(Mandatory)] [string]$Path)

    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Save-TextFile {
    param(
        [Parameter(Mandatory)] [string]$Path,
        [Parameter(Mandatory)] [string]$Content
    )

    Set-Content -Path $Path -Value $Content -Encoding utf8
}

function Export-UserEnvironment {
    $lines = [Environment]::GetEnvironmentVariables('User').GetEnumerator() |
        Sort-Object Key |
        ForEach-Object { '{0}={1}' -f $_.Key, $_.Value }

    return ($lines -join [Environment]::NewLine)
}

function Invoke-RobocopyMirror {
    param(
        [Parameter(Mandatory)] [string]$From,
        [Parameter(Mandatory)] [string]$To
    )

    $arguments = @(
        $From,
        $To,
        '/E',
        '/SL',
        '/SJ',
        '/COPY:DAT',
        '/DCOPY:DAT',
        '/R:1',
        '/W:1'
    )

    & robocopy @arguments
    $exitCode = $LASTEXITCODE

    if ($exitCode -gt 7) {
        throw ('robocopy failed with exit code ' + $exitCode)
    }
}

function Rewrite-UserEnvironmentReferences {
    param(
        [Parameter(Mandatory)] [string]$OldRoot,
        [Parameter(Mandatory)] [string]$NewRoot
    )

    $pattern = [regex]::Escape($OldRoot)
    $rewritten = @()

    $entries = [Environment]::GetEnvironmentVariables('User').GetEnumerator() |
        Where-Object {
            $_.Value -is [string] -and
            ($_.Value -like ('*' + $OldRoot + '*'))
        } |
        Sort-Object Key

    foreach ($entry in $entries) {
        $name = [string]$entry.Key
        $currentValue = [string]$entry.Value
        $updatedValue = $currentValue -replace $pattern, $NewRoot

        if ($updatedValue -ne $currentValue) {
            [Environment]::SetEnvironmentVariable($name, $updatedValue, 'User')
            Set-Item -Path ('Env:' + $name) -Value $updatedValue
            $rewritten += [pscustomobject]@{
                Name  = $name
                Value = $updatedValue
            }
        }
    }

    [Environment]::SetEnvironmentVariable('SCOOP', $NewRoot, 'User')
    $env:SCOOP = $NewRoot

    $machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $combinedPath = (@($machinePath, $userPath) | Where-Object { $_ }) -join ';'
    $env:PATH = $combinedPath

    return $rewritten
}

function Resolve-ScoopConfigPath {
    param(
        [Parameter(Mandatory)] [string]$SourceRoot,
        [Parameter(Mandatory)] [string]$TargetRoot
    )

    $candidates = @(
        (Join-Path $TargetRoot 'config.json'),
        (Join-Path $HOME '.config\scoop\config.json'),
        (Join-Path $SourceRoot 'config.json')
    )

    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            return $candidate
        }
    }

    return $null
}

function Rewrite-ConfigFileReferences {
    param(
        [Parameter(Mandatory)] [string]$Path,
        [Parameter(Mandatory)] [string]$OldRoot,
        [Parameter(Mandatory)] [string]$NewRoot
    )

    if (-not (Test-Path $Path)) {
        return $false
    }

    $pattern = [regex]::Escape($OldRoot)
    $original = Get-Content -Path $Path -Raw
    $updated = $original -replace $pattern, $NewRoot

    if ($updated -ne $original) {
        Set-Content -Path $Path -Value $updated -Encoding utf8
        return $true
    }

    return $false
}

function Get-UserEnvironmentReferences {
    param([Parameter(Mandatory)] [string]$Root)

    return @([Environment]::GetEnvironmentVariables('User').GetEnumerator() |
        Where-Object {
            $_.Value -is [string] -and
            ($_.Value -like ('*' + $Root + '*'))
        } |
        Sort-Object Key |
        Select-Object Key, Value)
}

$SourceRoot = Resolve-NormalizedPath $SourceRoot
$TargetRoot = Resolve-NormalizedPath $TargetRoot

if ([string]::IsNullOrWhiteSpace($BackupRoot)) {
    $targetDrive = [System.IO.Path]::GetPathRoot($TargetRoot)
    $BackupRoot = Join-Path $targetDrive 'Scoop-Migration-Backup'
}

$BackupRoot = Resolve-NormalizedPath $BackupRoot

if ($SourceRoot -eq $TargetRoot) {
    throw 'SourceRoot and TargetRoot must be different.'
}

if (-not (Test-Path ([System.IO.Path]::GetPathRoot($TargetRoot)))) {
    throw ('Target drive is not available: ' + [System.IO.Path]::GetPathRoot($TargetRoot))
}

$sourceExists = Test-Path $SourceRoot
$targetExists = Test-Path $TargetRoot

if (-not $sourceExists -and -not (Test-ScoopArtifacts -Root $TargetRoot)) {
    throw 'SourceRoot does not exist and TargetRoot does not look like a valid Scoop install.'
}

if (-not $SkipRunningProcessCheck -and $sourceExists) {
    Write-Step 'Checking for running processes from the old Scoop root'
    $running = @(Get-RunningProcessesFromRoot -Root $SourceRoot)
    if ($running.Count -gt 0) {
        $running | Format-Table -AutoSize | Out-String | Write-Host
        throw 'Close every Scoop-managed app from the old root and run the script again.'
    }
}

$initialScoopShim = Resolve-ScoopShim -PreferredRoots @($SourceRoot, $TargetRoot)

if (-not $SkipBackup) {
    Write-Step ('Creating logical backup in ' + $BackupRoot)
    Ensure-Directory -Path $BackupRoot

    $exportPath = Join-Path $BackupRoot 'scoopfile.json'
    $configTablePath = Join-Path $BackupRoot 'scoop-config-before.txt'
    $listPath = Join-Path $BackupRoot 'scoop-list-before.txt'
    $userEnvPath = Join-Path $BackupRoot 'user-env-before.txt'

    $exportContent = & $initialScoopShim export -c | Out-String
    Save-TextFile -Path $exportPath -Content $exportContent.Trim()

    $configContent = & $initialScoopShim config | Out-String
    Save-TextFile -Path $configTablePath -Content $configContent.TrimEnd()

    $listContent = & $initialScoopShim list | Out-String
    Save-TextFile -Path $listPath -Content $listContent.TrimEnd()

    Save-TextFile -Path $userEnvPath -Content (Export-UserEnvironment)
}

if ($sourceExists) {
    Write-Step ('Copying Scoop root to ' + $TargetRoot)
    Ensure-Directory -Path $TargetRoot
    Invoke-RobocopyMirror -From $SourceRoot -To $TargetRoot
}

if (-not (Test-ScoopArtifacts -Root $TargetRoot)) {
    throw 'TargetRoot does not contain the expected Scoop artifacts after copy.'
}

$configPath = Resolve-ScoopConfigPath -SourceRoot $SourceRoot -TargetRoot $TargetRoot
if ($null -ne $configPath) {
    Write-Step ('Rewriting old root references in ' + $configPath)
    $configUpdated = Rewrite-ConfigFileReferences -Path $configPath -OldRoot $SourceRoot -NewRoot $TargetRoot
    if (-not $configUpdated) {
        Write-Host 'No config file references needed rewriting.'
    }
}

Write-Step 'Rewriting persistent user environment variables'
$rewrittenVariables = @(Rewrite-UserEnvironmentReferences -OldRoot $SourceRoot -NewRoot $TargetRoot)
if ($rewrittenVariables.Count -gt 0) {
    $rewrittenVariables | Format-Table -AutoSize | Out-String | Write-Host
} else {
    Write-Host 'No user environment variables needed rewriting.'
}

$targetScoopShim = Resolve-ScoopShim -PreferredRoots @($TargetRoot, $SourceRoot)

Write-Step 'Updating Scoop root_path configuration'
Invoke-ScoopCommand -ScoopShim $targetScoopShim -Arguments @('config', 'root_path', $TargetRoot)

Write-Step 'Recreating shims, shortcuts, and app-managed environment variables'
Invoke-ScoopCommand -ScoopShim $targetScoopShim -Arguments @('reset', '-a')

Write-Step 'Running Scoop health checks'
Invoke-ScoopCommand -ScoopShim $targetScoopShim -Arguments @('checkup')

Write-Step 'Validating that no persistent user variables still point to the old root'
$remainingReferences = @(Get-UserEnvironmentReferences -Root $SourceRoot)
if ($remainingReferences.Count -gt 0) {
    $remainingReferences | Format-Table -AutoSize | Out-String | Write-Host
    throw 'Some user environment variables still point to the old Scoop root.'
}

Write-Step 'Validating the target Scoop install'
Invoke-ScoopCommand -ScoopShim $targetScoopShim -Arguments @('prefix', 'scoop')

if ($DeleteSourceRootAfterSuccess -and $sourceExists) {
    Write-Step ('Deleting old Scoop root at ' + $SourceRoot)
    Remove-Item -Path $SourceRoot -Recurse -Force
}

Write-Host ''
Write-Host 'Migration completed successfully.' -ForegroundColor Green
Write-Host ('Target root: ' + $TargetRoot)
Write-Host ('Scoop shim: ' + $targetScoopShim)

if (-not $DeleteSourceRootAfterSuccess -and $sourceExists) {
    Write-Host ('Old root was preserved: ' + $SourceRoot)
    Write-Host 'Delete it manually after opening a new PowerShell and confirming everything works from the new location.'
}