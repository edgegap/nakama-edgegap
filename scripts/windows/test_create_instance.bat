@echo off
echo Testing instance creation...

curl -X POST "http://localhost:7350/v2/rpc/instance_create?http_key=testkey123" ^
  -H "Content-Type: application/json" ^
  -d "{\"user_ids\":[\"test-user-1\"],\"metadata\":{\"test\":\"data\"}}"