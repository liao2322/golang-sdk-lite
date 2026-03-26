@echo off
setlocal

set "HALAL_CLIENT_ID=your_client_id"
set "HALAL_CLIENT_SECRET=your_client_secret"
set "HALAL_API_HOST=openapi.2dland.cn"
set "HALAL_WEB_ADDR=:8080"
set "HALAL_WEB_LINK_MODE=redirect"

if "%HALAL_CLIENT_ID%"=="" (
  echo HALAL_CLIENT_ID is required.
  exit /b 1
)

if "%HALAL_CLIENT_SECRET%"=="" (
  echo HALAL_CLIENT_SECRET is required.
  exit /b 1
)

go run ./cmd/webui
