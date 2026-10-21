import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1, // The fixture server resets its isolated database between tests.
  retries: 0, // Surface regressions; don't conceal intermittent failures with retries.
  timeout: 90_000,
  expect: { timeout: 15_000 },
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    actionTimeout: 10_000,
    baseURL: 'http://127.0.0.1:18199',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    ...devices['Desktop Chrome'],
    launchOptions: { args: ['--autoplay-policy=no-user-gesture-required'] },
  },
  webServer: {
    command: 'node e2e/server.mjs',
    url: 'http://127.0.0.1:18200/health',
    reuseExistingServer: false,
    timeout: 120_000,
  },
  globalTeardown: './e2e/teardown.ts',
});
