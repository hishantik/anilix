@echo off
setlocal enabledelayedexpansion

:: Anilix Installer for Windows
:: Stream Anime From Your Terminal
:: https://github.com/hishantik/anilix

set "REPO=hishantik/anilix"
set "BINARY=anilix.exe"
set "INSTALL_DIR=%LOCALAPPDATA%\anilix"
set "BASE_URL=https://github.com/%REPO%/releases/download"
set "API_URL=https://api.github.com/repos/%REPO%/releases/latest"

:: Generate ESC character for ANSI codes (Windows 10+)
for /f "delims=" %%a in ('echo prompt $E ^| cmd') do set "ESC=%%a"

:: ─── Material Design 3 Colors ────────────────────────────────────────────────
set "C_RESET=%ESC%[0m"
set "C_BOLD=%ESC%[1m"
set "C_PRIMARY=%ESC%[38;2;208;188;255m"
set "C_PRIMARY_BRIGHT=%ESC%[38;2;220;199;255m"
set "C_PINK=%ESC%[38;2;217;70;239m"
set "C_SUCCESS=%ESC%[38;2;134;217;164m"
set "C_WARNING=%ESC%[38;2;245;194;107m"
set "C_ERROR=%ESC%[38;2;242;184;181m"
set "C_MUTED=%ESC%[38;2;202;196;208m"
set "C_OUTLINE=%ESC%[38;2;147;143;153m"

:: Parse arguments
set "VERSION="
set "ACTION=install"

:parse_args
if "%~1"=="" goto :main
if /i "%~1"=="--version" (
    set "VERSION=%~2"
    shift & shift
    goto :parse_args
)
if /i "%~1"=="--update" (
    set "ACTION=update"
    shift
    goto :parse_args
)
if /i "%~1"=="--uninstall" (
    set "ACTION=uninstall"
    shift
    goto :parse_args
)
if /i "%~1"=="--help" goto :usage
shift
goto :parse_args

:usage
echo.
echo  Anilix Installer for Windows
echo  Stream Anime From Your Terminal
echo.
echo  Usage:
echo    install.bat                    Install latest version
echo    install.bat --update           Update to latest version
echo    install.bat --version v1.0.0   Install specific version
echo    install.bat --uninstall        Remove anilix
echo    install.bat --help             Show this help
echo.
exit /b 0

:main
if "%ACTION%"=="uninstall" goto :uninstall

:: Check for curl
where curl >nul 2>&1
if errorlevel 1 (
    echo.
    echo  %C_ERROR%[error]%C_RESET% curl not found. Please install curl or use Windows 10+.
    exit /b 1
)

:: Print banner
call :print_banner

:: Fetch latest version if not specified
if "%VERSION%"=="" (
    echo.
    call :info "Fetching latest version..."
    for /f "tokens=2 delims=:, " %%a in ('curl -fsSL "%API_URL%" ^| findstr /i "tag_name"') do (
        set "VERSION=%%~a"
    )
    set "VERSION=!VERSION:"=!"
    if "!VERSION!"=="" (
        call :die "Failed to fetch latest version from GitHub"
    )
    call :info "Latest version: !VERSION!"
)

echo.
echo  %C_PRIMARY%[info]%C_RESET% Fetching !VERSION! for windows/amd64...

:: Build URL
set "ARCHIVE=anilix_windows_amd64.zip"
set "URL=%BASE_URL%/!VERSION!/%ARCHIVE%"

:: Create temp directory
set "TMPDIR=%TEMP%\anilix_install_%RANDOM%"
mkdir "!TMPDIR!" 2>nul

:: Download
call :info "Downloading..."
echo  !URL!
echo.

curl -fSL -o "!TMPDIR!\!ARCHIVE!" "!URL!"
if errorlevel 1 (
    call :die "Download failed. Check if version !VERSION! exists."
)

:: Extract
call :info "Extracting archive..."
powershell -NoProfile -Command "Expand-Archive -Path '!TMPDIR!\!ARCHIVE!' -DestinationPath '!TMPDIR!\extracted' -Force"
if errorlevel 1 (
    call :die "Extraction failed. Is PowerShell available?"
)

:: Install
if not exist "!INSTALL_DIR!" mkdir "!INSTALL_DIR!"
copy /y "!TMPDIR!\extracted\!BINARY!" "!INSTALL_DIR!\!BINARY!" >nul
if errorlevel 1 (
    call :die "Failed to copy binary to !INSTALL_DIR!"
)

:: Cleanup
del /q "!TMPDIR!\!ARCHIVE!" 2>nul
rmdir /s /q "!TMPDIR!" 2>nul

call :separator
echo.

:: PATH check
call :info "Checking PATH..."
echo !PATH! | findstr /i /c:"!INSTALL_DIR!" >nul
if not errorlevel 1 (
    call :success "Already in PATH"
) else (
    call :info "Adding to PATH..."
    setx PATH "!PATH!;!INSTALL_DIR!" >nul 2>&1
    if errorlevel 1 (
        call :warning "Could not add to PATH automatically."
        echo.
        echo  Add to PATH manually: !INSTALL_DIR!
        echo  Or run: setx PATH "%%PATH%%;!INSTALL_DIR!"
    ) else (
        call :success "Added to PATH. Restart your terminal to use anilix."
    )
)

echo.
call :check_player
call :separator
echo.
echo  %C_PINK%Run: anilix%C_RESET%  (restart terminal first if PATH was updated)
echo.
exit /b 0

:: ─── Uninstall ───────────────────────────────────────────────────────────────

:uninstall
call :print_banner
echo.
set "INSTALL_DIR=%LOCALAPPDATA%\anilix"

call :info "Uninstalling anilix..."
if exist "!INSTALL_DIR!\!BINARY!" (
    del "!INSTALL_DIR!\!BINARY!"
    call :success "Removed !INSTALL_DIR!\!BINARY!"
) else (
    call :warning "anilix not found at !INSTALL_DIR!"
)

if exist "!INSTALL_DIR!" rmdir "!INSTALL_DIR!" 2>nul
echo.
call :success "Uninstalled."
exit /b 0

:: ─── Player Check ────────────────────────────────────────────────────────────

:check_player
where mpv >nul 2>&1
if not errorlevel 1 (
    call :success "Found player: mpv"
    exit /b 0
)
where vlc >nul 2>&1
if not errorlevel 1 (
    call :success "Found player: vlc"
    exit /b 0
)

call :warning "No supported video player found. Anilix needs one of: mpv, vlc"
echo.
echo  Install suggestions:
echo    %C_PRIMARY%winget install mpv%C_RESET%     (recommended)
echo    or download from: https://mpv.io/installation/
exit /b 0

:: ─── UI Helpers ─────────────────────────────────────────────────────────────

:print_banner
echo.
echo  %C_PRIMARY_BRIGHT%ANILIX%C_RESET%  Stream Anime From Your Terminal
echo  %C_OUTLINE%-------------------------------------------------------------%C_RESET
exit /b 0

:info
echo  %C_PRIMARY%[info]%C_RESET% %~1
exit /b 0

:success
echo  %C_SUCCESS%[ok]%C_RESET% %~1
exit /b 0

:warning
echo  %C_WARNING%[warn]%C_RESET% %~1
exit /b 0

:die
echo  %C_ERROR%[error]%C_RESET% %~1
if exist "!TMPDIR!" rmdir /s /q "!TMPDIR!" 2>nul
exit /b 1

:separator
echo  %C_OUTLINE%-------------------------------------------------------------%C_RESET
exit /b 0
