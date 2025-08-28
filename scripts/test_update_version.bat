@echo off
REM Script to update Edgegap deployment version via S2S RPC
REM Usage: test_update_version.bat <version> [http_key] [nakama_url]
REM Example: test_update_version.bat "v1.0.0"

if "%1"=="" (
    echo Usage: %0 ^<version^> [http_key] [nakama_url]
    echo Example: %0 "v1.0.0"
    exit /b 1
)

set VERSION=%~1
set HTTP_KEY=testkey123
set NAKAMA_URL=http://localhost:7350

if not "%2"=="" set HTTP_KEY=%2
if not "%3"=="" set NAKAMA_URL=%3

echo Updating Edgegap version to: %VERSION%
echo Using HTTP key: %HTTP_KEY%
echo Nakama URL: %NAKAMA_URL%

curl "%NAKAMA_URL%/v2/rpc/update_edgegap_version?http_key=%HTTP_KEY%&unwrap" ^
  -H "Content-Type: application/json" ^
  -H "Accept: application/json" ^
  -d "{\"version\": \"%VERSION%\"}"

echo.
echo.
echo Verifying update...
curl "%NAKAMA_URL%/v2/rpc/get_edgegap_version?http_key=%HTTP_KEY%&unwrap" ^
  -H "Content-Type: application/json" ^
  -H "Accept: application/json" ^
  -d "{}"

echo.