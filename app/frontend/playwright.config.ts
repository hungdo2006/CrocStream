import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  timeout: 20000,
  workers: 1,
  reporter: 'list',
  use: {
    channel: 'msedge',
    baseURL: 'http://127.0.0.1:9245',
    screenshot: 'off', video: 'off', trace: 'off',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://127.0.0.1:9245',
    reuseExistingServer: false,
    timeout: 30000,
  },
})
