@echo off
REM GoTunnel Build Script Launcher for Windows
REM This script launches the PowerShell build script

setlocal

set SCRIPT_DIR=%~dp0

REM Check if PowerShell is available
where powershell >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo ERROR: PowerShell is not available
    exit /b 1
)

REM Pass all arguments to PowerShell script
powershell -ExecutionPolicy Bypass -File "%SCRIPT_DIR%build.ps1" %*

endlocal
