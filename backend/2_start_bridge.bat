@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

if not exist "config.yml" (
  echo [ERROR] config.yml not found in: %cd%
  echo.
  pause
  exit /b 1
)

if not exist "mb_control.ps1" (
  echo [ERROR] mb_control.ps1 not found in: %cd%
  echo.
  pause
  exit /b 1
)

echo ============================================
echo   Starting Moon Bridge + 模型配置面板
echo ============================================
echo   - 桥服务:   127.0.0.1:38440
echo   - 配置面板: http://127.0.0.1:38450  (浏览器会自动弹出)
echo.
echo   在弹出的网页里选模型 / 填 Key，点“保存并重启”即可切换模型。
echo   KEEP THIS WINDOW OPEN. 关闭本窗口或 Ctrl+C 将同时停止桥服务。
echo ============================================
echo.

powershell -NoProfile -ExecutionPolicy Bypass -File ".\mb_control.ps1"

echo.
echo [控制服务器已退出。] 查看上方日志了解原因。
pause
endlocal
