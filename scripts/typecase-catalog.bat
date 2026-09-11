@echo off
setlocal

rem Live Typecase widget catalog (http://localhost:8080/)
cd /d "%~dp0..\typecase"
if errorlevel 1 (
  echo Failed to change directory to typecase module.
  exit /b 1
)

if "%PORT%"=="" set "PORT=8080"

echo Starting Typecase catalog on http://localhost:%PORT%/
echo   /        - with tc.js behavior
echo   /static  - markup and styles only
echo.
go run ./cmd/catalog
