@echo off
setlocal

rem Live Typecase assembled screens (http://localhost:8081/)
cd /d "%~dp0..\typecase"
if errorlevel 1 (
  echo Failed to change directory to typecase module.
  exit /b 1
)

if "%PORT%"=="" set "PORT=8081"

echo Starting Typecase screens on http://localhost:%PORT%/
echo   /             - pick a screen from the nav
echo   /notes        - Notes list
echo   /notes-read   - Notes read view
echo   /notes-edit   - Notes edit view
echo   /notes-quick  - Notes quick capture
echo   /files        - Files
echo   /contacts     - Contacts
echo   /io           - Import and export
echo   /duplicates   - Duplicates
echo   /states       - UI states
echo   Append ?theme=dark for dark mode.
echo.

rem Open the browser once the server responds.
start "" powershell -NoProfile -WindowStyle Hidden -Command ^
  "$u='http://localhost:%PORT%/';" ^
  "for($i=0;$i -lt 60;$i++){" ^
  "  try{$null=Invoke-WebRequest -Uri $u -UseBasicParsing -TimeoutSec 1;" ^
  "  Start-Process $u; exit 0}catch{Start-Sleep -Milliseconds 500}" ^
  "}"

go run ./cmd/screens
if errorlevel 1 (
  echo.
  echo Server exited with an error.
  pause
)
