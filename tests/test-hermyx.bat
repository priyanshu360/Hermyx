@echo off
setlocal enabledelayedexpansion

set HERMYX_URL=http://localhost:8080

echo Testing /hello endpoint (should cache)
curl -i %HERMYX_URL%/hello
echo.
timeout /t 1 >nul
curl -i %HERMYX_URL%/hello
echo.
echo.

echo Testing /time endpoint (cache TTL 10s)
curl -i %HERMYX_URL%/time
echo.
timeout /t 2 >nul
curl -i %HERMYX_URL%/time
echo.
echo.

echo Testing /delay endpoint (5 second delay)
echo Start time: %time%
curl -i %HERMYX_URL%/delay
echo End time: %time%
echo.
echo.

echo Testing cached /delay endpoint (should be instant)
echo Start time: %time%
curl -i %HERMYX_URL%/delay
echo End time: %time%
echo.
echo.

echo Testing exceeded content size /exceed endpoint
echo Start time: %time%
curl -i %HERMYX_URL%/exceed
echo End time: %time%
echo.
echo.

echo Testing /echo endpoint with query param (cache key includes query)
curl -i "%HERMYX_URL%/echo?msg=first"
echo.
curl -i "%HERMYX_URL%/echo?msg=second"
echo.
curl -i "%HERMYX_URL%/echo?msg=first"
echo.

