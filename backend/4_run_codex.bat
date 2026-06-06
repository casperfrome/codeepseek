@echo off
chcp 65001 >nul
setlocal

set "CODEX_HOME=D:\moon_bridge_main\.codex"

rem WORKDIR = folder Codex will operate in.
rem   - double-click: uses this script's folder
rem   - or drag a project folder onto this .bat
rem   - or: 4_run_codex.bat "D:\path\to\project"
set "WORKDIR=%~1"
if "%WORKDIR%"=="" set "WORKDIR=%cd%"

if not exist "%CODEX_HOME%\config.toml" (
  echo [ERROR] Not found: %CODEX_HOME%\config.toml
  echo         Run 3_gen_codex_config.bat first.
  echo.
  pause
  exit /b 1
)

echo ============================================
echo   Launching Codex  (via Moon Bridge -^> DeepSeek)
echo ============================================
echo   CODEX_HOME = %CODEX_HOME%
echo   WORKDIR    = %WORKDIR%
echo   NOTE: keep 2_start_bridge.bat running in another window.
echo ============================================
echo.

codex --cd "%WORKDIR%"

echo.
pause
endlocal
