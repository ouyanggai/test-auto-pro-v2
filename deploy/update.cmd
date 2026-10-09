@echo off
chcp 65001 >nul
set PYTHONDONTWRITEBYTECODE=1
set PYTHONUTF8=1
cd /d "%~dp0.."
python -u deploy\update.py %*
set "result=%errorlevel%"
if not "%result%"=="0" echo 更新发布失败，请查看上方提示和日志。
pause
exit /b %result%
