@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

echo Building moonbridge.exe (first run downloads Go modules, please wait)...
go build -o moonbridge.exe ./cmd/moonbridge
if errorlevel 1 (
  echo.
  echo [FAILED] Build error. See the Go messages above.
) else (
  echo.
  echo [OK] Built: %cd%\moonbridge.exe
)
echo.
pause
endlocal
