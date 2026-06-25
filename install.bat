@echo off
setlocal enabledelayedexpansion

:: Anilix Installer for Windows
:: Usage: install.bat [--version v1.0.0] [--uninstall] [--help]

set "REPO=hishantik/anilix"
set "BINARY=anilix.exe"
set "INSTALL_DIR=%LOCALAPPDATA%\anilix"
set "BASE_URL=https://github.com/%REPO%/releases/download"
set "API_URL=https://api.github.com/repos/%REPO%/releases/latest"

:: Enable Virtual Terminal Processing for ANSI colors (Windows 10+)
for /f "tokens=3" %%a in ('reg query "HKCU\Console" /v VirtualTerminalLevel 2^>nul') do set "VTL=%%a"
if not "!VTL!"=="1" (
    reg add "HKCU\Console" /v VirtualTerminalLevel /t REG_DWORD /d 1 /f >nul 2>&1
)

:: Generate ESC character for ANSI codes using PowerShell (more reliable than cmd)
for /f "delims=" %%a in ('powershell -NoProfile -Command "Write-Host ([char]27)"') do set "ESC=%%a"

:: Fallback if ESC generation failed
if "!ESC!"=="" set "ESC="

:: ANSI color codes matching Anilix theme
set "C_BLUE=%ESC%[0;34m"
set "C_GREEN=%ESC%[0;32m"
set "C_YELLOW=%ESC%[0;33m"
set "C_RED=%ESC%[0;31m"
set "C_PURPLE=%ESC%[38;2;157;78;221m"
set "C_RESET=%ESC%[0m"

:: Parse arguments
set "VERSION="
set "UNINSTALL=0"

:parse_args
if "%~1"=="" goto :main
if /i "%~1"=="--version" (
    set "VERSION=%~2"
    shift & shift
    goto :parse_args
)
if /i "%~1"=="--uninstall" (
    set "UNINSTALL=1"
    shift
    goto :parse_args
)
if /i "%~1"=="--help" (
    echo Anilix Installer for Windows
    echo.
    echo Usage:
    echo   install.bat                   Install latest version
    echo   install.bat --version v1.0.0  Install specific version
    echo   install.bat --uninstall       Remove anilix
    echo   install.bat --help            Show this help
    exit /b 0
)
shift
goto :parse_args

:main
if "%UNINSTALL%"=="1" goto :uninstall

:: Check for curl
where curl >nul 2>&1
if errorlevel 1 (
    echo %C_RED%[error]%C_RESET% curl not found. Please install curl or use Windows 10+.
    exit /b 1
)

:: Fetch latest version if not specified
if "%VERSION%"=="" (
    echo %C_BLUE%[info]%C_RESET% Fetching latest version...
    for /f "tokens=2 delims=:, " %%a in ('curl -fsSL "%API_URL%" ^| findstr /i "tag_name"') do (
        set "VERSION=%%~a"
    )
    set "VERSION=!VERSION:"=!"
    if "!VERSION!"=="" (
        echo %C_RED%[error]%C_RESET% Failed to fetch latest version.
        exit /b 1
    )
)

echo %C_BLUE%[info]%C_RESET% Installing !VERSION! for windows/amd64...

:: Create temp directory
set "TMPDIR=%TEMP%\anilix_install_%RANDOM%"
mkdir "%TMPDIR%" 2>nul

:: Download with curl progress bar
set "ARCHIVE=anilix_windows_amd64.zip"
set "URL=%BASE_URL%/!VERSION!/%ARCHIVE%"
echo %C_BLUE%[info]%C_RESET% Downloading !URL!...

curl -fSL --progress-bar -o "!TMPDIR!\!ARCHIVE!" "!URL!"

if errorlevel 1 (
    echo %C_RED%[error]%C_RESET% Download failed. Check if version !VERSION! exists.
    rmdir /s /q "%TMPDIR%" 2>nul
    exit /b 1
)

:: Extract
echo %C_BLUE%[info]%C_RESET% Extracting...
powershell -NoProfile -Command "Expand-Archive -Path '!TMPDIR!\!ARCHIVE!' -DestinationPath '!TMPDIR!\extracted' -Force"
if errorlevel 1 (
    echo %C_RED%[error]%C_RESET% Extraction failed.
    rmdir /s /q "%TMPDIR%" 2>nul
    exit /b 1
)

:: Install
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
copy /y "%TMPDIR%\extracted\%BINARY%" "%INSTALL_DIR%\%BINARY%" >nul
if errorlevel 1 (
    echo %C_RED%[error]%C_RESET% Failed to copy binary to %INSTALL_DIR%
    rmdir /s /q "%TMPDIR%" 2>nul
    exit /b 1
)

:: Cleanup
rmdir /s /q "%TMPDIR%" 2>nul

:: Add to PATH if not already there
echo "%PATH%" | findstr /i /c:"%INSTALL_DIR%" >nul
if errorlevel 1 (
    echo %C_BLUE%[info]%C_RESET% Adding %INSTALL_DIR% to PATH...
    setx PATH "%PATH%;%INSTALL_DIR%" >nul 2>&1
    if errorlevel 1 (
        echo %C_YELLOW%[warn]%C_RESET% Failed to add to PATH automatically.
        echo %C_YELLOW%[warn]%C_RESET% Add this to your PATH manually: %INSTALL_DIR%
    ) else (
        echo %C_GREEN%[ok]%C_RESET% Added to PATH. Restart your terminal to use 'anilix'.
    )
)

echo.
echo %C_GREEN%[ok]%C_RESET% Installed !VERSION! to %INSTALL_DIR%\%BINARY%
echo.
echo Run 'anilix' to get started (restart terminal first if PATH was updated).
exit /b 0

:uninstall
echo %C_BLUE%[info]%C_RESET% Uninstalling anilix...
if exist "%INSTALL_DIR%\%BINARY%" (
    del "%INSTALL_DIR%\%BINARY%"
    echo %C_GREEN%[ok]%C_RESET% Removed %INSTALL_DIR%\%BINARY%
) else (
    echo %C_YELLOW%[warn]%C_RESET% anilix not found at %INSTALL_DIR%
)

echo "%PATH%" | findstr /i /c:"%INSTALL_DIR%" >nul
if not errorlevel 1 (
    echo %C_YELLOW%[warn]%C_RESET% Remove %INSTALL_DIR% from your PATH manually.
)

rmdir "%INSTALL_DIR%" 2>nul
echo %C_GREEN%[ok]%C_RESET% Uninstalled.
exit /b 0
