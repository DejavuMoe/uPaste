import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

const configDir = path.dirname(fileURLToPath(import.meta.url));
const serverScript = path.join(configDir, '..', 'scripts', 'e2e-production-server.sh');

// Qualifies the real single-file production binary: WebServer runs the
// repository script against the embedded bundle (no Vite).
export default defineConfig({
  testDir: './e2e-production',
  use: { baseURL: 'http://127.0.0.1:4174', trace: 'off', video: 'off', screenshot: 'only-on-failure' },
  reporter: 'line',
  workers: 1,
  fullyParallel: false,
  webServer: {
    command: `bash ${serverScript}`,
    port: 4174,
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
