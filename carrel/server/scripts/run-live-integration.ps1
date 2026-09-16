# Live S-30.2 integration tests from Windows.
#
# The credentials are read by the tests themselves from dev/credentials.txt
# (integration/live_creds.go), so nothing is parsed twice here: a second
# reader of that file is a second thing to break when the file is edited.
# Set CARREL_* in the environment to override what the file says.
#
# Radicale is left out: it runs on the Linux machine (scripts/linux-gates.sh).
param(
    [string]$Run = 'Baikal|SFTPGo|Yandex|MailRu|SoGo|OpenCloud'
)
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

$credsPath = Join-Path $PSScriptRoot "../../../dev/credentials.txt"
if (-not (Test-Path $credsPath)) {
    Write-Warning "no dev/credentials.txt: every live test will skip itself unless CARREL_* is set"
}

go test -tags=integration -count=1 -run $Run ./integration/... -v
