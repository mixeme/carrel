@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem Start Carrel locally and open it in the default browser.
rem Stops a running instance first, then waits for /healthz.
rem
rem Usage: scripts\run.bat [port] [data-dir] [nobrowser]
rem   port      default 8080
rem   data-dir  default dev/data
rem   nobrowser skip opening the browser

set "ROOT=%~dp0.."
cd /d "%ROOT%" || exit /b 1
set "ROOT=%CD%"

set "PORT=8080"
set "DATADIR=dev/data"
set "NOBROWSER=0"
set "BINARY=%ROOT%\dist\carrel.exe"
set "STARTUP_TIMEOUT=30"

if not "%~1"=="" set "PORT=%~1"
if not "%~2"=="" set "DATADIR=%~2"
if /I "%~3"=="nobrowser" set "NOBROWSER=1"

if not exist "%BINARY%" (
    echo Binary not found: %BINARY%
    echo Run scripts\build.bat first.
    exit /b 1
)

if not exist "%ROOT%\dev" mkdir "%ROOT%\dev"
if not exist "%ROOT%\%DATADIR%" mkdir "%ROOT%\%DATADIR%"

set "BASE_URL=http://127.0.0.1:%PORT%/"
set "HEALTH_URL=http://127.0.0.1:%PORT%/healthz"

call :StopCarrel
call :WaitPortFree
if errorlevel 1 exit /b 1

set "CARREL_DATA_DIR=%DATADIR%"
set "CARREL_PORT=%PORT%"

echo Starting Carrel...
echo   binary:  %BINARY%
echo   data:    %DATADIR%  (%ROOT%\%DATADIR%)
echo   url:     %BASE_URL%

start "Carrel" /D "%ROOT%" "%BINARY%"

call :WaitReady
if errorlevel 1 exit /b 1

echo Carrel is ready.

if "%NOBROWSER%"=="0" (
    start "" "%BASE_URL%"
    echo Opened %BASE_URL% in the default browser.
)

exit /b 0

:StopCarrel
set "STOPPED=0"

tasklist /FI "IMAGENAME eq carrel.exe" 2>nul | find /I "carrel.exe" >nul
if not errorlevel 1 (
    echo Stopping Carrel...
    taskkill /IM carrel.exe /F >nul 2>&1
    set "STOPPED=1"
)

for /f "tokens=5" %%P in ('netstat -ano ^| findstr ":%PORT% " ^| findstr /I "LISTENING"') do (
    echo Stopping process on port %PORT% ^(PID %%P^)...
    taskkill /PID %%P /F >nul 2>&1
    set "STOPPED=1"
)

if "!STOPPED!"=="0" echo Carrel is not running.
exit /b 0

:WaitPortFree
set /a TRIES=0
:PortFreeLoop
netstat -ano | findstr ":%PORT% " | findstr /I "LISTENING" >nul
if errorlevel 1 exit /b 0
set /a TRIES+=1
if !TRIES! GEQ 50 (
    echo Port %PORT% is still in use after stopping Carrel.
    exit /b 1
)
timeout /t 1 /nobreak >nul
goto PortFreeLoop

:WaitReady
where curl >nul 2>&1
if errorlevel 1 (
    echo curl is not in PATH; waiting 3 seconds before opening the browser...
    timeout /t 3 /nobreak >nul
    exit /b 0
)

set /a ATTEMPTS=0
:ReadyLoop
curl -sf -o nul "%HEALTH_URL%"
if not errorlevel 1 exit /b 0
set /a ATTEMPTS+=1
if !ATTEMPTS! GEQ %STARTUP_TIMEOUT% (
    echo Carrel did not become ready within %STARTUP_TIMEOUT%s. Check the server window for errors.
    exit /b 1
)
timeout /t 1 /nobreak >nul
goto ReadyLoop
