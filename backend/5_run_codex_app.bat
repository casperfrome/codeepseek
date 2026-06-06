@echo off
chcp 65001 >nul
setlocal

set "CODEX_HOME=D:\moon_bridge_main\.codex"

rem Workspace folder to open. Double-click = this folder; or drag a
rem project folder onto this .bat; or: 5_run_codex_app.bat "D:\path\to\project"
set "WORKDIR=%~1"
if "%WORKDIR%"=="" set "WORKDIR=%cd%"

rem ============================================================
rem  Launch the Codex DESKTOP APP ("codex app") on Moon Bridge -> DeepSeek.
rem  Requires codex with the "app" subcommand (>= 0.137).
rem  If missing, upgrade:  npm install -g @openai/codex@latest
rem  NOTE: the FIRST "codex app" downloads/installs the Codex Desktop app.
rem ============================================================

if not exist "%CODEX_HOME%\config.toml" (
  echo [ERROR] Not found: %CODEX_HOME%\config.toml
  echo         Run 3_gen_codex_config.bat first.
  echo.
  pause
  exit /b 1
)

rem Capability check: is "app" listed as a codex subcommand?
rem (codex app --help can't be used: --help always exits 0.)
codex --help 2>nul | findstr /B /C:"  app " >nul
if errorlevel 1 (
  echo [ERROR] This codex build has no "codex app" command.
  echo         Upgrade:  npm install -g @openai/codex@latest
  echo         Then double-click this script again.
  echo.
  codex --version
  echo.
  pause
  exit /b 1
)

echo ============================================
echo   Launching Codex DESKTOP APP
echo   (via Moon Bridge 127.0.0.1:38440 -^> DeepSeek)
echo ============================================
echo   CODEX_HOME = %CODEX_HOME%
echo   WORKDIR    = %WORKDIR%
echo   NOTE: keep 2_start_bridge.bat running in another window.
echo   NOTE: first launch will download/install the Codex Desktop app.
echo ============================================
echo.

codex app "%WORKDIR%"

echo.
echo [Codex app exited.]
pause
endlocal
