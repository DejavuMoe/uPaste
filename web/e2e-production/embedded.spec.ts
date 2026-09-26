import { expect, test, type Page } from '@playwright/test';

// The existing web/e2e suite qualifies behavior through the Vite development
// server. This focused suite qualifies the same product flows served by the
// real self-contained Go binary with the production bundle embedded.

const secret = 'embedded production qualification';

test.afterEach(async () => {
  // Keep process-local create/mutation rate limits deterministic across tests.
  await new Promise((resolve) => setTimeout(resolve, 6500));
});

function collectCspViolations(page: Page): string[] {
  const violations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' && /content security policy/i.test(message.text())) {
      violations.push(message.text());
    }
  });
  return violations;
}

test('embedded index and hashed assets load with strict production headers', async ({ page }) => {
  const response = await page.goto('/');
  expect(response).not.toBeNull();
  expect(response!.status()).toBe(200);

  const headers = response!.headers();
  expect(headers['content-type']).toContain('text/html');
  expect(headers['cache-control']).toBe('no-store');
  expect(headers['x-content-type-options']).toBe('nosniff');
  expect(headers['referrer-policy']).toBe('no-referrer');
  expect(headers['x-frame-options']).toBe('DENY');

  const csp = headers['content-security-policy'] ?? '';
  expect(csp).toContain("default-src 'self'");
  expect(csp).toContain("script-src 'self'");
  expect(csp).toContain("style-src 'self'");
  expect(csp).toContain("connect-src 'self'");
  expect(csp).toContain("frame-ancestors 'none'");
  expect(csp).not.toContain('unsafe-eval');
  expect(csp).not.toContain('unsafe-inline');
  expect(csp).not.toContain('*');

  await expect(page.getByRole('heading', { name: '新建分享' })).toBeVisible();

  const scriptSrc = await page.locator('script[src^="/assets/"]').first().getAttribute('src');
  const cssHref = await page.locator('link[rel="stylesheet"][href^="/assets/"]').first().getAttribute('href');
  expect(scriptSrc).toBeTruthy();
  expect(cssHref).toBeTruthy();

  const javascript = await page.request.get(scriptSrc!);
  expect(javascript.status()).toBe(200);
  expect(javascript.headers()['cache-control']).toBe('public, max-age=31536000, immutable');
  expect(javascript.headers()['content-type']).toContain('javascript');
  expect(javascript.headers()['x-content-type-options']).toBe('nosniff');

  const css = await page.request.get(cssHref!);
  expect(css.status()).toBe(200);
  expect(css.headers()['cache-control']).toBe('public, max-age=31536000, immutable');
  expect(css.headers()['content-type']).toContain('css');
});

test('direct viewer and management routes resolve while unrelated routes stay 404', async ({ page, request }) => {
  const created = await request.post('/api/v1/shares', {
    data: {
      payload_kind: 'TEXT',
      privacy_mode: 'STANDARD',
      text: { format: 'PLAIN', content: secret },
      expires_at: null,
    },
  });
  expect(created.status()).toBe(201);
  const payload = await created.json();
  const id = payload.share.id as string;

  const viewerResponse = await page.goto(`/s/${id}`);
  expect(viewerResponse!.status()).toBe(200);
  await expect(page.getByText(secret)).toBeVisible();

  const managementResponse = await page.goto(`/manage/${id}`);
  expect(managementResponse!.status()).toBe(200);
  await expect(page.getByRole('heading', { name: 'Management token' })).toBeVisible();

  for (const path of ['/does-not-exist', '/admin', '/login', '/dashboard']) {
    const response = await request.get(path);
    expect(response.status(), `expected 404 for ${path}`).toBe(404);
  }
});

test('standard text create, viewer, and owner management work from the embedded binary', async ({ page }) => {
  const violations = collectCspViolations(page);

  await page.goto('/');
  await page.getByRole('button', { name: 'EN' }).click();
  await page.getByRole('textbox', { name: 'Content' }).fill(secret);
  const createResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && response.url().includes('/api/v1/shares'),
  );
  await page.getByRole('button', { name: 'Create share' }).click();
  const created = await (await createResponse).json();
  await expect(page.getByRole('heading', { name: 'Share created' })).toBeVisible();

  await page.getByRole('button', { name: 'Open share' }).click();
  await expect(page.getByText(secret)).toBeVisible();

  await page.goto(`/manage/${created.share.id}`);
  await page.getByLabel('Management token').fill(created.owner_token);
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.getByLabel('Editor')).toHaveValue(secret);

  await page.getByLabel('Editor').fill('embedded management updated');
  const patchResponse = page.waitForResponse((response) => response.request().method() === 'PATCH');
  await page.getByRole('button', { name: 'Save changes' }).click();
  expect((await patchResponse).status()).toBe(200);
  await expect(page.getByText('Changes saved')).toBeVisible();

  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(page.getByText('embedded management updated')).toBeVisible();
  expect(violations).toEqual([]);
});

test('file shares download only from the separate file origin', async ({ page, request }) => {
  const violations = collectCspViolations(page);
  const bytes = Buffer.from('embedded production file body');

  await page.goto('/');
  await page.getByRole('button', { name: 'EN' }).click();
  await page.getByRole('tab', { name: 'File' }).click();
  await page.locator('input[type=file]').setInputFiles({
    name: 'embedded-production.txt',
    mimeType: 'text/plain',
    buffer: bytes,
  });

  const createResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && response.url().includes('/api/v1/shares'),
  );
  await page.getByRole('button', { name: 'Create share' }).click();
  const created = await (await createResponse).json();
  await expect(page.getByRole('heading', { name: 'Share created' })).toBeVisible();

  const downloadUrl = created.share.file.download_url as string;
  expect(downloadUrl).toBe(`http://127.0.0.1:4191/f/${created.share.id}`);

  const download = await request.get(downloadUrl);
  expect(download.status()).toBe(200);
  expect(await download.body()).toEqual(bytes);
  expect(download.headers()['content-disposition']).toContain('attachment');
  expect(download.headers()['x-content-type-options']).toBe('nosniff');
  expect(download.headers()['access-control-allow-origin']).toBeUndefined();

  expect((await request.get(`/f/${created.share.id}`)).status()).toBe(404);
  expect(violations).toEqual([]);
});

test('encrypted text is encrypted and decrypted in the browser on the embedded origin', async ({ page, request }) => {
  const content = 'embedded encrypted secret';

  await page.goto('/');
  await page.getByRole('button', { name: 'EN' }).click();
  await page.getByRole('radio', { name: 'Encrypted', exact: true }).click();
  await page.getByRole('textbox', { name: 'Content' }).fill(content);
  const createResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && response.url().includes('/api/v1/shares'),
  );
  await page.getByRole('button', { name: 'Create share' }).click();
  const created = await (await createResponse).json();
  expect(created.share.privacy_mode).toBe('ENCRYPTED');

  const shareLink = await page.getByLabel('Share link').inputValue();
  expect(shareLink).toContain('#up_e1_');
  expect(shareLink).not.toContain(created.owner_token);

  await page.getByRole('button', { name: 'Open share' }).click();
  await expect(page.getByText(content)).toBeVisible();
  expect(page.url()).toContain('#up_e1_');

  const apiResponse = await request.get(`/api/v1/shares/${created.share.id}`);
  const apiBody = await apiResponse.text();
  expect(apiBody).not.toContain(content);
  expect(apiBody).not.toContain('#up_e1_');
});

test('missing assets, unknown routes, and wrong methods fail closed', async ({ request }) => {
  expect((await request.get('/assets/does-not-exist.js')).status()).toBe(404);
  expect((await request.get('/does-not-exist')).status()).toBe(404);
  expect((await request.get('/f/AAAAAAAAAAAAAAAAAAAAAA')).status()).toBe(404);

  const postRoot = await request.post('/', { data: {} });
  expect(postRoot.status()).toBe(405);
  expect(postRoot.headers()['allow']).toBe('GET, HEAD');

  expect((await request.post('/healthz')).status()).toBe(405);

  const apiMissing = await request.get('/api/v1/shares/AAAAAAAAAAAAAAAAAAAAAA');
  expect(apiMissing.status()).toBe(404);
  expect((await apiMissing.json()).error.code).toBe('not_found');

  expect((await request.get('/raw/AAAAAAAAAAAAAAAAAAAAAA')).status()).toBe(404);
});
