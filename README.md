# golang-sdk-light
This is a lightweight version of the halal cloud storage API SDK, which does not depend on other heavyweight libraries.

## web ui
This repository now also includes a lightweight MVP web app that wraps the SDK and provides:

- device-code login
- responsive file browsing for desktop / tablet / mobile
- recent files and trash views
- offline task list and creation
- media preview with browser-native video/audio/image playback
- `302` redirect endpoints for play and download

### run the web ui
Set the required environment variables and start the server.

Windows `.bat`:

```bat
set HALAL_CLIENT_ID=your_client_id
set HALAL_CLIENT_SECRET=your_client_secret
set HALAL_API_HOST=openapi.2dland.cn
set HALAL_WEB_ADDR=:8080
set HALAL_WEB_LINK_MODE=redirect
run-webui.bat
```

Direct `go run`:

```bash
HALAL_CLIENT_ID=your_client_id
HALAL_CLIENT_SECRET=your_client_secret
HALAL_API_HOST=openapi.2dland.cn
HALAL_WEB_ADDR=:8080
HALAL_WEB_LINK_MODE=redirect
go run ./cmd/webui
```

Then open `http://localhost:8080`.

### docker

Build image:

```bash
docker build -t halalcloud/golang-sdk-lite-webui:latest .
```

Run container:

```bash
docker run -d \
  --name halalcloud-webui \
  -p 8080:8080 \
  -e HALAL_CLIENT_ID=your_client_id \
  -e HALAL_CLIENT_SECRET=your_client_secret \
  -e HALAL_API_HOST=openapi.2dland.cn \
  -e HALAL_WEB_ADDR=:8080 \
  -e HALAL_WEB_LINK_MODE=redirect \
  -e HALAL_DEFAULT_ROOT=/ \
  halalcloud/golang-sdk-lite-webui:latest
```

Or use `docker-compose.yml` and edit the environment values first.

### github actions build

This repository also includes a GitHub Actions workflow:

- `.github/workflows/docker-image.yml`

It builds and pushes the image to GitHub Container Registry:

```text
ghcr.io/<your-github-username>/golang-sdk-lite-webui:latest
```

How to use:

1. Push this repository to GitHub
2. Open the `build-docker-image` workflow
3. Run it manually, or push to `main`
4. Pull the image on your VPS

Example on VPS:

```bash
docker pull ghcr.io/your-github-username/golang-sdk-lite-webui:latest
docker run -d \
  --name halalcloud-webui \
  -p 8080:8080 \
  -e HALAL_CLIENT_ID=your_client_id \
  -e HALAL_CLIENT_SECRET=your_client_secret \
  -e HALAL_API_HOST=openapi.2dland.cn \
  -e HALAL_WEB_ADDR=:8080 \
  -e HALAL_WEB_LINK_MODE=redirect \
  -e HALAL_DEFAULT_ROOT=/ \
  ghcr.io/your-github-username/golang-sdk-lite-webui:latest
```

`HALAL_WEB_LINK_MODE` supports:

- `redirect`: OpenList-style `302` redirect to the direct link
- `proxy`: OpenList-style slice proxy playback/download path

### pwa and ios shell

The web UI now includes:

- `/manifest.webmanifest`
- `/sw.js`
- `/favicon.svg`

Capacitor iOS shell scaffolding lives in:

- `mobile/capacitor/package.json`
- `mobile/capacitor/capacitor.config.ts`
- `.github/workflows/ios-capacitor-ipa.yml`

Before building the iOS shell:

1. Deploy the web UI to an HTTPS domain on your VPS
2. Update `mobile/capacitor/capacitor.config.ts`
3. Add iOS signing secrets to GitHub Actions
4. Run the `build-ios-ipa` workflow manually

### main backend endpoints

- `POST /api/auth/device-code/start`
- `GET /api/auth/device-code/status`
- `GET /api/auth/session`
- `GET /api/user/me`
- `GET /api/user/quota`
- `GET /api/files`
- `GET /api/files/recent`
- `GET /api/files/trash`
- `POST /api/files/create`
- `POST /api/files/rename`
- `POST /api/files/trash-action`
- `POST /api/files/recover`
- `POST /api/files/delete`
- `POST /api/files/upload-task`
- `GET /api/files/play/:identity`
- `GET /api/files/download/:identity`
- `GET /api/offline/tasks`
- `POST /api/offline/tasks`
- `DELETE /api/offline/tasks/:identity`

# docs
https://douyu666.feishu.cn/wiki/TFLswuavUiTwADkWnY1cafqunKh

# website
https://2dland.yuque.com/r/organizations/homepage

# support
qingzhen10086@gmail.com
