import type { CapacitorConfig } from "@capacitor/cli";

const config: CapacitorConfig = {
  appId: "cn.halalcloud.webui",
  appName: "Halal Cloud",
  webDir: "www",
  bundledWebRuntime: false,
  server: {
    url: "https://your-vps-domain.example.com",
    cleartext: false,
    allowNavigation: ["your-vps-domain.example.com"]
  },
  ios: {
    contentInset: "automatic",
    scrollEnabled: true
  }
};

export default config;
