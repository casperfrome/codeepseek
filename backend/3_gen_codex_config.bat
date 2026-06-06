@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

set "CODEX_HOME=D:\moon_bridge_main\.codex"
if not exist "%CODEX_HOME%" mkdir "%CODEX_HOME%"

rem Use .\ prefix: this environment has NoDefaultCurrentDirectoryInExePath=1,
rem so cmd will NOT find moonbridge.exe by bare name even from its own folder.
set "MB=.\moonbridge.exe"
if not exist "moonbridge.exe" set "MB=go run ./cmd/moonbridge"

echo [1/2] Reading Codex model alias from config.yml ...
set "MB_MODEL="
for /f "delims=" %%i in ('%MB% -print-codex-model -config "config.yml"') do set "MB_MODEL=%%i"
if "%MB_MODEL%"=="" (
  echo [FAILED] Could not read model alias. Make sure config.yml loads OK
  echo          (try 2_start_bridge.bat to see the error^).
  echo.
  pause
  exit /b 1
)
echo       model alias = %MB_MODEL%
echo.

echo [2/2] Writing config.toml + models_catalog.json to:
echo       %CODEX_HOME%
%MB% -print-codex-config "%MB_MODEL%" -codex-base-url "http://127.0.0.1:38440/v1" -codex-home "%CODEX_HOME%" -config "config.yml" > "%CODEX_HOME%\config.toml"
if errorlevel 1 (
  echo.
  echo [FAILED] Could not generate Codex config.
) else (
  echo.
  echo [OK] Generated:
  echo      %CODEX_HOME%\config.toml
  echo      %CODEX_HOME%\models_catalog.json
)
echo.
pause
endlocal
