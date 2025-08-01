@echo off
setlocal enabledelayedexpansion

REM Project metadata
set APP_NAME=hermyx
set MAIN_PATH=.\cmd\main.go
set OUTPUT_DIR=.\bin

REM Use environment or detect
if not defined GOOS (
  for /f %%i in ('go env GOOS') do set GOOS=%%i
)
if not defined GOARCH (
  for /f %%i in ('go env GOARCH') do set GOARCH=%%i
)

REM Set .exe extension for Windows
set EXT=
if "%GOOS%"=="windows" (
  set EXT=.exe
)

set OUTPUT_PATH=%OUTPUT_DIR%\%APP_NAME%-%GOOS%-%GOARCH%%EXT%

if not exist %OUTPUT_DIR% (
  mkdir %OUTPUT_DIR%
)

echo 🔧 Building %APP_NAME% for %GOOS%/%GOARCH%...
go build -ldflags="-s -w" -o %OUTPUT_PATH% %MAIN_PATH%

if %ERRORLEVEL% NEQ 0 (
  echo ❌ Build failed!
  exit /b 1
)

echo ✅ Build successful: %OUTPUT_PATH%
