@echo off
setlocal EnableExtensions

rem Build carrel.exe into dist/ with version metadata from git.
rem Usage: scripts\build.bat [output-path]

set "ROOT=%~dp0.."
cd /d "%ROOT%" || exit /b 1
set "ROOT=%CD%"

where go >nul 2>&1
if errorlevel 1 (
    echo Go is not in PATH. Install Go 1.22 or newer and try again.
    exit /b 1
)

if "%~1"=="" (
    set "OUTPUT=%ROOT%\dist\carrel.exe"
) else (
    set "OUTPUT=%~1"
    if not "%~f1"=="%~1" set "OUTPUT=%ROOT%\%~1"
)

for %%I in ("%OUTPUT%") do set "OUTPUT_DIR=%%~dpI"
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%" 2>nul

set "VERSION=0.8.0"
set "COMMIT=unknown"

where git >nul 2>&1
if not errorlevel 1 (
    for /f "delims=" %%V in ('git describe --tags --always --dirty 2^>nul') do set "VERSION=%%V"
    for /f "delims=" %%C in ('git rev-parse --short HEAD 2^>nul') do set "COMMIT=%%C"
)

set "CGO_ENABLED=0"

echo Building %OUTPUT%
echo   version: %VERSION%
echo   commit:  %COMMIT%

go build -ldflags="-s -w -X main.version=%VERSION% -X main.commit=%COMMIT%" -o "%OUTPUT%" ./cmd/carrel
if errorlevel 1 (
    echo go build failed.
    exit /b 1
)

echo Done.
exit /b 0
