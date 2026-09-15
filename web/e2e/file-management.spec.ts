import { expect, test, type Page, type Request } from '@playwright/test';
import { createHash } from 'node:crypto';

const bytes = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3, 4]);
const name = 'file-e2e-fixture.png';
const expectedSha256 = createHash('sha256').update(bytes).digest('hex');

test.afterEach(async () => { await new Promise(r => setTimeout(r, 6500)); });

async function create(page: Page) {
  await page.goto('/');
  await page.getByRole('tab', { name: 'File' }).click();
  const post = page.waitForRequest(r => r.method() === 'POST' && r.url().includes('/api/v1/shares'));
  const res = page.waitForResponse(r => r.request().method() === 'POST' && r.url().includes('/api/v1/shares'));
  await page.locator('input[type=file]').setInputFiles({ name, mimeType: 'text/html', buffer: bytes });
  await page.getByRole('button', { name: 'Create share' }).click();
  const r = await res, data = await r.json();
  await expect(page.getByRole('heading', { name: 'Share created' })).toBeVisible();
  return { request: await post, response: r, data, id: data.share.id as string, token: data.owner_token as string, url: data.share.file.download_url as string };
}
async function manage(page: Page) { await page.getByRole('button', { name: 'Manage share' }).click(); }
function security(h: Record<string, string>) {
  expect(h['x-content-type-options']).toBe('nosniff');
  expect(h['cache-control']).toContain('no-store');
  expect(h['referrer-policy']).toBe('no-referrer');
  expect(h['x-frame-options']).toBe('DENY');
  expect(h['content-security-policy']).toContain('sandbox');
}
async function confidentialRequest(r: Request, token: string) {
  expect(Boolean((await r.allHeaders()).authorization)).toBe(true);
  expect(r.url().includes(token)).toBe(false);
  expect((r.postData() || '').includes(token)).toBe(false); // JSON mutation, never multipart upload.
}
async function confidentialBrowser(page: Page, token: string) {
  const surfaces = await page.evaluate(() => ({ pathname: location.pathname, search: location.search, hash: location.hash, state: history.state, local: { ...localStorage }, session: { ...sessionStorage }, cookie: document.cookie }));
  expect(surfaces.hash).toBe('');
  expect(JSON.stringify(surfaces).includes(token)).toBe(false);
}
async function viewerMetadata(page: Page, url: string) {
  await expect(page.getByText(name, { exact: true })).toBeVisible();
  await expect(page.getByText('12 B', { exact: true })).toBeVisible();
  await expect(page.getByText('image/png', { exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Download file' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Download file' })).toHaveAttribute('href', url);
}

test('real multipart creation uses server-sniffed metadata', async ({ page }) => {
  const x = await create(page), h = await x.request.allHeaders();
  expect(h['content-type']).toMatch(/^multipart\/form-data; boundary=/);
  expect(x.response.status()).toBe(201);
  expect(x.data.share).toMatchObject({ payload_kind: 'FILE', privacy_mode: 'STANDARD', file: { filename: name, size: bytes.length, media_type: 'image/png' } });
  expect(x.data.share.file.sha256).toMatch(/^[a-f0-9]{64}$/);
  expect(x.data.share.file.sha256).toBe(expectedSha256);
  expect(x.data.share.file.media_type).not.toBe('text/html');
  expect(x.url).toBe(`http://127.0.0.1:4181/f/${x.id}`);
  const url = new URL(x.url);
  expect({ protocol: url.protocol, hostname: url.hostname, port: url.port, pathname: url.pathname, search: url.search, hash: url.hash }).toEqual({ protocol: 'http:', hostname: '127.0.0.1', port: '4181', pathname: `/f/${x.id}`, search: '', hash: '' });
  expect(x.url.includes(x.token)).toBe(false);
});

test('viewer is metadata-only and does not auto-fetch bytes', async ({ page }) => {
  const x = await create(page);
  let hits = 0;
  page.on('request', r => { if (r.url() === x.url) hits++; });
  await page.getByRole('button', { name: 'Open share' }).click();
  await viewerMetadata(page, x.url);
  expect(hits).toBe(0);
  expect(await page.locator(`iframe[src="${x.url}"],img[src="${x.url}"],video[src="${x.url}"],audio[src="${x.url}"],object[data="${x.url}"],embed[src="${x.url}"]`).count()).toBe(0);
});

test('file origin is exact-byte attachment with security headers', async ({ page, request }) => {
  const x = await create(page), r = await request.get(x.url), h = r.headers();
  expect(r.status()).toBe(200);
  expect(Buffer.from(await r.body())).toEqual(bytes);
  expect(h['content-type']).toContain('image/png');
  expect(h['content-disposition']).toContain('attachment');
  expect(h['etag']).toMatch(/^"[a-f0-9]{64}"$/);
  expect(h['etag']).toBe(`"${expectedSha256}"`);
  security(h);
  expect(h['access-control-allow-origin']).toBeUndefined();
});

test('HEAD and Range delivery behavior', async ({ page, request }) => {
  const x = await create(page), head = await request.head(x.url), range = await request.get(x.url, { headers: { Range: 'bytes=0-3' } });
  expect(head.status()).toBe(200);
  expect(await head.body()).toHaveLength(0);
  expect(head.headers()['content-type']).toContain('image/png');
  expect(head.headers()['content-disposition']).toContain('attachment');
  expect(head.headers()['etag']).toBe(`"${expectedSha256}"`);
  security(head.headers());
  expect(range.status()).toBe(206);
  expect(Buffer.from(await range.body())).toEqual(bytes.subarray(0, 4));
  expect(range.headers()['content-range']).toMatch(/^bytes 0-3\//);
  expect(range.headers()['x-content-type-options']).toBe('nosniff');
  expect(range.headers()['cache-control']).toContain('no-store');
});

test('management is expiry/delete-only and exact expiration mutations', async ({ page }) => {
  const x = await create(page);
  await manage(page);
  await expect(page.getByRole('heading', { name: 'Manage share' })).toBeVisible();
  for (const text of ['Filename', name, 'Size', '12 B', 'Media type', 'image/png']) await expect(page.getByText(text, { exact: true })).toBeVisible();
  await expect(page.getByLabel('Expiration', { exact: true })).toBeVisible();
  for (const button of ['Update expiration', 'Cancel', 'Delete share']) await expect(page.getByRole('button', { name: button, exact: true })).toBeVisible();
  for (const label of ['Editor', 'Format', 'File upload']) await expect(page.getByLabel(label, { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Choose file|Remove selected file/ })).toHaveCount(0);
  await expect(page.locator('input[type=file]')).toHaveCount(0);
  await confidentialBrowser(page, x.token);
  const p = page.waitForRequest(r => r.method() === 'PATCH');
  const saved = page.waitForResponse(r => r.request().method() === 'PATCH');
  await page.getByLabel('Expiration', { exact: true }).selectOption('1d');
  await page.getByRole('button', { name: 'Update expiration' }).click();
  const r = await p, body = JSON.parse(r.postData()!);
  expect(Object.keys(body)).toEqual(['expires_at']);
  expect(Date.parse(body.expires_at)).toBeGreaterThan(Date.now());
  await confidentialRequest(r, x.token);
  expect((await saved).status()).toBe(200);
  await expect(page.getByText('Changes saved')).toBeVisible();
  const never = page.waitForRequest(r => r.method() === 'PATCH');
  const neverSaved = page.waitForResponse(r => r.request().method() === 'PATCH');
  await page.getByLabel('Expiration', { exact: true }).selectOption('never');
  await page.getByRole('button', { name: 'Update expiration' }).click();
  const n = await never;
  expect(JSON.parse(n.postData()!)).toEqual({ expires_at: null });
  await confidentialRequest(n, x.token);
  expect((await neverSaved).status()).toBe(200);
  await confidentialBrowser(page, x.token);
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await viewerMetadata(page, x.url);
});

test('delete removes API metadata and file object', async ({ page, request }) => {
  const x = await create(page);
  await manage(page);
  await page.getByRole('button', { name: 'Delete share' }).click();
  await page.getByRole('button', { name: 'Cancel' }).last().click();
  await expect(page.getByRole('button', { name: 'Delete share' })).toBeVisible();
  await page.getByRole('button', { name: 'Delete share' }).click();
  const deleted = page.waitForRequest(r => r.method() === 'DELETE');
  await page.getByRole('button', { name: 'Delete permanently' }).click();
  await confidentialRequest(await deleted, x.token);
  await expect(page.getByText('Share deleted')).toBeVisible();
  await confidentialBrowser(page, x.token);
  expect((await request.get(`http://127.0.0.1:4180/api/v1/shares/${x.id}`)).status()).toBe(404);
  const get = await request.get(x.url), head = await request.head(x.url);
  expect(get.status()).toBe(404);
  expect(head.status()).toBe(404);
  security(get.headers());
});
