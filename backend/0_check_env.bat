@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

echo ============================================
echo   Moon Bridge / Codex x DeepSeek - Env Check
echo ============================================
echo.

echo [1/3] Go (need 1.25+)
go version || echo   ^>^> Go NOT found. Install: https://go.dev/dl/  (amd64 64-bit recommended)
echo.

echo [2/3] Node.js / npm
node --version || echo   ^>^> Node.js NOT found. Install: https://nodejs.org
npm --version 2>nul
echo.

echo [3/3] Codex CLI
codex --version || echo   ^>^> codex NOT found. Run: npm install -g @openai/codex
echo.

echo --------------------------------------------
echo If all three show a version number, you are ready.
echo --------------------------------------------
echo.
pause
endlocal
