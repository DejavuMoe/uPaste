import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

const configDir = path.dirname(fileURLToPath(import.meta.url));
const serverScript = path.join(configDir, '..', 'scripts', 'e2e-public-server.sh');

export default defineConfig({
  testDir: './e2e-public',
  use: { baseURL: 'http://127.0.0.1:4175', trace: 'off', video: 'off', screenshot: 'only-on-failure' },
  reporter: 'line',
  workers: 1,
  fullyParallel: false,
  webServer: {
    command: `bash ${serverScript}`,
    port: 4175,
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
