import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  timeout: 30_000,
  use: {
    baseURL: 'http://127.0.0.1:18080',
    channel: 'chrome',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'desktop',
      use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 1000 } },
    },
    { name: 'mobile', use: { ...devices['iPhone 13'], defaultBrowserType: 'chromium' } },
  ],
  webServer: {
    command: 'go run ../cmd/api',
    url: 'http://127.0.0.1:18080/readyz',
    reuseExistingServer: false,
    env: {
      HTTP_ADDR: '127.0.0.1:18080',
      WEB_DIR: 'dist',
      DATABASE_DSN: 'file:gachaslop-e2e?mode=memory&cache=shared',
      GOCACHE: '/tmp/gachaslop-go-cache',
    },
  },
});
