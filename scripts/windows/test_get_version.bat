@echo off
REM Script to get Edgegap deployment version via S2S RPC
REM Usage: test_get_version.bat [http_key] [nakama_url]

set HTTP_KEY=testkey123
set NAKAMA_URL=http://localhost:7350

if not "%1"=="" set HTTP_KEY=%1
if not "%2"=="" set NAKAMA_URL=%2

echo Getting Edgegap version from %NAKAMA_URL%...
curl "%NAKAMA_URL%/v2/rpc/get_edgegap_version?http_key=%HTTP_KEY%&unwrap" ^
  -H "Content-Type: application/json" ^
  -H "Accept: application/json" ^
  -d "{}"

echo.