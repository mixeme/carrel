# Live S-30.2 integration tests. Reads dev/credentials.txt (not in git).
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
$credsPath = (Resolve-Path (Join-Path $PSScriptRoot "../../../dev/credentials.txt")).Path
if (-not (Test-Path $credsPath)) { throw "missing $credsPath" }

$section = ""
Get-Content $credsPath | ForEach-Object {
    $line = $_.TrimEnd()
    switch -Regex ($line) {
        '^Baikal DAV$' { $section = 'baikal'; return }
        '^SFTPGo WebDAV$' { $section = 'sftpgo'; return }
        '^$' { $section = ''; return }
    }
    if ($section -eq 'baikal') {
        if ($line -match '^https?://') { $env:CARREL_BAIKAL_URL = $line }
        elseif ($line -match '^login:\s*(.+)$') { $env:CARREL_BAIKAL_USER = $Matches[1] }
        elseif ($line -match '^password:\s*(.+)$') { $env:CARREL_BAIKAL_PASS = $Matches[1] }
    }
    if ($section -eq 'sftpgo') {
        if ($line -match '^https?://') { $env:CARREL_SFTPGO_URL = $line }
        elseif ($line -match '^login:\s*(.+)$') { $env:CARREL_SFTPGO_USER = $Matches[1] }
        elseif ($line -match '^password:\s*(.+)$') { $env:CARREL_SFTPGO_PASS = $Matches[1] }
    }
}

go test -tags=integration -count=1 -run 'SFTPGo|Baikal' ./integration/... -v
