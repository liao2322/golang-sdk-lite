# Capacitor Shell

This folder contains a lightweight Capacitor shell for wrapping the deployed web app as an iOS app.

## 1. Edit the target URL

Update `capacitor.config.ts`:

- replace `https://your-vps-domain.example.com` with your deployed HTTPS domain
- update `appId` if needed

## 2. Install dependencies

```bash
npm install
```

## 3. Add the iOS platform

```bash
npm run cap:add:ios
```

## 4. Sync config changes

```bash
npm run cap:sync
```

## 5. Build iOS on cloud macOS

Use GitHub Actions or another macOS CI service to archive and sign the generated Xcode project.

## Notes

- This project is intended for sideloaded/self-signed `.ipa` distribution
- It points to the online VPS deployment, so web updates do not require rebuilding the app
