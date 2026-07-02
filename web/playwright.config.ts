import { defineConfig, devices } from '@playwright/test';

const backendPort = Number(process.env.E2E_BACKEND_PORT ?? 18080);
const frontendPort = Number(process.env.E2E_FRONTEND_PORT ?? 15173);
const backendURL = `http://127.0.0.1:${backendPort}`;
const frontendURL = `http://127.0.0.1:${frontendPort}`;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [['line'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: frontendURL,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chrome',
      use: {
        ...devices['Desktop Chrome'],
        channel: 'chrome',
      },
    },
  ],
  webServer: [
    {
      command: 'go run ./cmd/accounting-server',
      cwd: '../backend',
      env: {
        ACCOUNTING_ADDR: `127.0.0.1:${backendPort}`,
        ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED: 'false',
        ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED: 'true',
        ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL: `${backendURL}/api/health`,
        ACCOUNTING_AUTH_PASSKEY_RP_ID: '127.0.0.1',
        ACCOUNTING_AUTH_PASSKEY_RP_ORIGIN: frontendURL,
        ACCOUNTING_AUTH_SESSION_COOKIE_SECURE: 'false',
        ACCOUNTING_WEB_DIST_DIR: './missing-web-dist',
      },
      url: `${backendURL}/api/health`,
      timeout: 120_000,
      reuseExistingServer: !process.env.CI,
      stdout: 'ignore',
      stderr: 'pipe',
    },
    {
      command: `npm run dev -- --host 127.0.0.1 --port ${frontendPort}`,
      env: {
        VITE_API_BASE_URL: backendURL,
      },
      url: frontendURL,
      timeout: 120_000,
      reuseExistingServer: !process.env.CI,
      stdout: 'ignore',
      stderr: 'pipe',
    },
  ],
});
