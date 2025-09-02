@echo off
echo Testing initial version trigger by attempting instance creation...
echo.

REM This will fail but should trigger the initial version logic
curl -X POST "http://localhost:7350/v2/rpc/instance_create" ^
  -H "Content-Type: application/json" ^
  -H "Authorization: Bearer dummy_token" ^
  -d "{\"max_players\":4}"

echo.
echo.
echo Now checking if initial version was stored...
curl -X POST "http://localhost:7350/v2/rpc/get_edgegap_version?http_key=testkey123&unwrap" ^
  -H "Content-Type: application/json" ^
  -d "{}"