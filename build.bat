@echo off
cd /d "%~dp0"
set "PATH=%PATH%;C:\Program Files\Go\bin"
go build -ldflags="-X main.CompatiblePluginVersion=1.3.0" -o CassFr.exe ./cmd/assetreuploader
if errorlevel 1 (
  echo.
  echo Build failed. Copy the error above and send it over.
  pause
  exit /b 1
)
echo.
echo Done: CassFr.exe is in this folder. Run it to start CassFr 1.3.0.
pause
