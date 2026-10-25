import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  timeout: 15000,
  workers: 1,
  use: {
    baseURL: "http://127.0.0.1:15473",
    headless: true,
    launchOptions: {
      ...(process.env.CHROME_PATH
        ? { executablePath: process.env.CHROME_PATH }
        : {}),
      args: ["--no-sandbox"],
    },
  },
  webServer: {
    command: "npm run dev -- --host 127.0.0.1 --port 15473 --strictPort",
    url: "http://127.0.0.1:15473",
    reuseExistingServer: false,
  },
});
