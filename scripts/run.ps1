#Requires -Version 5.1
<#
.SYNOPSIS
    Start Carrel locally and open it in the default browser.

.DESCRIPTION
    If Carrel is already running (same binary or the configured port), stops it
    first, then starts a fresh process and waits until /healthz responds before
    opening the service URL.

    By default, state is stored in dev/data/ under the repository (same layout as
    dev/local-testing.md). The dev/ folder is not tracked by git.

.EXAMPLE
    .\scripts\run.ps1

.EXAMPLE
    .\scripts\run.ps1 -Port 9090 -DataDir dev/data
#>
[CmdletBinding()]
param(
    [int]$Port = 8080,
    [string]$DataDir = "",
    [string]$Binary = "",
    [switch]$NoBrowser,
    [int]$StartupTimeoutSec = 30
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

if (-not $Binary) {
    $Binary = Join-Path $Root "dist\carrel.exe"
}
if (-not (Test-Path -LiteralPath $Binary)) {
    throw "Binary not found: $Binary. Run .\scripts\build.ps1 first."
}
$Binary = (Resolve-Path -LiteralPath $Binary).Path

# Local state lives in dev/data/ (see dev/local-testing.md). dev/ itself is gitignored.
$DevRoot = Join-Path $Root "dev"
if (-not $DataDir) {
    $DataDir = "dev/data"
}

if ([System.IO.Path]::IsPathRooted($DataDir)) {
    $DataDirPath = $DataDir
    $DataDirForEnv = $DataDir
}
else {
    $DataDirPath = Join-Path $Root $DataDir
    $DataDirForEnv = $DataDir
}

New-Item -ItemType Directory -Force -Path $DevRoot | Out-Null
New-Item -ItemType Directory -Force -Path $DataDirPath | Out-Null

$BaseUrl = "http://127.0.0.1:$Port/"
$HealthUrl = "http://127.0.0.1:$Port/healthz"

function Get-ListenerProcessIds {
    param([int]$ListenPort)

    if (-not (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue)) {
        return @()
    }

    return @(
        Get-NetTCPConnection -LocalPort $ListenPort -State Listen -ErrorAction SilentlyContinue |
            Select-Object -ExpandProperty OwningProcess -Unique
    ) | Where-Object { $_ -gt 0 }
}

function Stop-CarrelInstance {
    $stopped = $false

    Get-Process -Name "carrel" -ErrorAction SilentlyContinue | ForEach-Object {
        $procPath = $_.Path
        if ($procPath -and ((Resolve-Path -LiteralPath $procPath).Path -eq $Binary)) {
            Write-Host "Stopping Carrel (PID $($_.Id))..."
            Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
            $stopped = $true
        }
    }

    foreach ($processId in (Get-ListenerProcessIds -ListenPort $Port)) {
        try {
            $proc = Get-Process -Id $processId -ErrorAction Stop
        }
        catch {
            continue
        }

        if ($proc.ProcessName -ne "carrel") {
            throw "Port $Port is already used by $($proc.ProcessName) (PID $processId). Stop it manually or choose another port with -Port."
        }

        if ($proc.Path -and ((Resolve-Path -LiteralPath $proc.Path).Path -ne $Binary)) {
            Write-Host "Stopping other Carrel instance on port $Port (PID $processId)..."
        }
        else {
            Write-Host "Stopping Carrel on port $Port (PID $processId)..."
        }

        Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
        $stopped = $true
    }

    if (-not $stopped) {
        Write-Host "Carrel is not running."
        return
    }

    $deadline = (Get-Date).AddSeconds(10)
    while ((Get-Date) -lt $deadline) {
        if ((Get-ListenerProcessIds -ListenPort $Port).Count -eq 0) {
            return
        }
        Start-Sleep -Milliseconds 200
    }

    throw "Port $Port is still in use after stopping Carrel."
}

function Wait-CarrelReady {
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSec)

    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -Uri $HealthUrl -UseBasicParsing -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                return
            }
        }
        catch {
            Start-Sleep -Milliseconds 300
        }
    }

    throw "Carrel did not become ready within ${StartupTimeoutSec}s. Check the server window for errors."
}

Stop-CarrelInstance

$env:CARREL_DATA_DIR = $DataDirForEnv
$env:CARREL_PORT = "$Port"

Write-Host "Starting Carrel..."
Write-Host "  binary:  $Binary"
Write-Host "  data:    $DataDirForEnv  ($DataDirPath)"
Write-Host "  url:     $BaseUrl"

Start-Process -FilePath $Binary -WorkingDirectory $Root -WindowStyle Normal | Out-Null

Wait-CarrelReady
Write-Host "Carrel is ready."

if (-not $NoBrowser) {
    Start-Process $BaseUrl
    Write-Host "Opened $BaseUrl in the default browser."
}
