import { expect, test } from '@playwright/test';

test.afterEach(async () => {
  await new Promise((resolve) => setTimeout(resolve, 6500));
});

// Qualifies the strict production CSP against the dynamic File progress UI.
// CDP throttling makes the 2 MiB upload observable so the progress element is
// exercised instead of being skipped by an instantaneous loopback transfer.
test('slow upload advances progress without weakening strict CSP', async ({ page, context }) => {
  test.setTimeout(120_000);

  const cspViolations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' && /content security policy/i.test(message.text())) {
      cspViolations.push(message.text());
    }
  });

  await page.goto('/');
  await page.getByRole('button', { name: 'EN' }).click();
  await expect(page.getByRole('heading', { name: 'New share' })).toBeVisible();

  const client = await context.newCDPSession(page);
  await client.send('Network.enable');
  await client.send('Network.emulateNetworkConditions', {
    offline: false,
    latency: 10,
    downloadThroughput: -1,
    uploadThroughput: 200 * 1024,
    connectionType: 'cellular3g',
  });

  const bytes = Buffer.alloc(2 * 1024 * 1024, 0x61);
  await page.getByRole('tab', { name: 'File' }).click();
  await page.locator('input[type=file]').setInputFiles({
    name: 'slow-upload.bin',
    mimeType: 'application/octet-stream',
    buffer: bytes,
  });
  await page.getByRole('button', { name: 'Create share' }).click();

  const fill = page.locator('.progress-bar-fill');
  await expect(fill).toBeVisible({ timeout: 20_000 });
  const firstWidth = await fill.evaluate((element) => parseFloat(getComputedStyle(element).width));
  expect(firstWidth).toBeGreaterThanOrEqual(0);
  expect(await fill.getAttribute('style')).toMatch(/width/);

  await expect
    .poll(async () => parseFloat(await fill.evaluate((element) => getComputedStyle(element).width)), {
      timeout: 30_000,
    })
    .toBeGreaterThan(firstWidth);

  await expect(page.getByRole('heading', { name: 'Share created' })).toBeVisible({ timeout: 90_000 });
  expect(cspViolations).toEqual([]);
});
