import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

const configDir = path.dirname(fileURLToPath(import.meta.url));
const serverScript = path.join(configDir, '..', 'scripts', 'e2e-cap-server.sh');

export default defineConfig({
  testDir: './e2e-cap',
  use: { baseURL: 'http://127.0.0.1:4176', trace: 'off', video: 'off', screenshot: 'only-on-failure' },
  reporter: 'line',
  workers: 1,
  fullyParallel: false,
  webServer: {
    command: `bash ${serverScript}`,
    port: 4176,
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
