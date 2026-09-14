import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  use: { baseURL: 'http://127.0.0.1:4173', trace: 'off', video: 'off', screenshot: 'only-on-failure' },
  reporter: 'line',
  webServer: { command: 'bash scripts/e2e-server.sh', port: 4173, reuseExistingServer: false, timeout: 90_000 },
});
