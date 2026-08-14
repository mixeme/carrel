#Requires -Version 5.1
<#
.SYNOPSIS
    Build the Carrel binary for the current platform.

.DESCRIPTION
    Runs go build with version metadata from git and writes carrel.exe to dist/
    under the repository root. Data files are not created here; use run.ps1 to
    start the server.

.EXAMPLE
    .\scripts\build.ps1
#>
[CmdletBinding()]
param(
    [string]$Output = ""
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is not in PATH. Install Go 1.22 or newer and try again."
}

if (-not $Output) {
    $Output = Join-Path $Root "dist\carrel.exe"
}

$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}

$Version = "0.1.0"
$Commit = "unknown"

if (Get-Command git -ErrorAction SilentlyContinue) {
    $gitVersion = git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -eq 0 -and $gitVersion) {
        $Version = $gitVersion
    }

    $gitCommit = git rev-parse --short HEAD 2>$null
    if ($LASTEXITCODE -eq 0 -and $gitCommit) {
        $Commit = $gitCommit
    }
}

$env:CGO_ENABLED = "0"

$ldflags = "-s -w -X main.version=$Version -X main.commit=$Commit"

Write-Host "Building $Output"
Write-Host "  version: $Version"
Write-Host "  commit:  $Commit"

& go build -ldflags $ldflags -o $Output ./cmd/carrel
if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}

Write-Host "Done."
