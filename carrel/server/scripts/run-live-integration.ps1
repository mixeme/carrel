# Live S-30.2 integration tests from Windows.
#
# The credentials are read by the tests themselves (integration/live_creds.go)
# from myenv/carrel/credentials.txt in the nearest folder above the checkout,
# or from the file CARREL_CREDENTIALS names, so nothing is parsed twice here: a
# second reader of that file is a second thing to break when the file is
# edited. Set CARREL_* in the environment to override what the file says.
#
# Radicale is left out: it runs on the Linux machine (scripts/linux-gates.sh).
param(
    [string]$Run = 'Baikal|SFTPGo|Yandex|MailRu|SoGo|OpenCloud'
)
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

$credsPath = $env:CARREL_CREDENTIALS
if (-not $credsPath) {
    $dir = $PSScriptRoot
    while ($dir) {
        $candidate = Join-Path $dir "myenv/carrel/credentials.txt"
        if (Test-Path $candidate) { $credsPath = $candidate; break }
        $dir = Split-Path $dir -Parent
    }
}
if (-not $credsPath -or -not (Test-Path $credsPath)) {
    Write-Warning "no myenv/carrel/credentials.txt: every live test will skip itself unless CARREL_* is set"
}

go test -tags=integration -count=1 -run $Run ./integration/... -v
