@echo off
setlocal

set "HALAL_CLIENT_ID=puc_816b1a334bf84a00a4382aa143580588_mm7gf2bw_v1"
set "HALAL_CLIENT_SECRET=732a1ec1dbe04c818b264171b1bc9eda"
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
