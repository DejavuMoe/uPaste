import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

const fakeToken = 'cap-test-token';
const adminToken = `up_a1_${'A'.repeat(43)}`;

async function createTextShare(request: APIRequestContext, content: string): Promise<string> {
  const response = await request.post('/api/v1/shares', {
    headers: { 'X-uPaste-Challenge': fakeToken },
    data: {
      payload_kind: 'TEXT',
      privacy_mode: 'STANDARD',
      text: { format: 'PLAIN', content },
      expires_at: null,
    },
  });
  expect(response.status(), await response.text()).toBe(201);
  const body = await response.json();
  return body.share.id as string;
}

async function signInAdmin(page: Page): Promise<void> {
  await page.getByLabel('Admin token').fill(adminToken);
  const loginResponse = page.waitForResponse((response) => response.request().method() === 'POST' && response.url().includes('/api/v1/admin/session'));
  await page.getByRole('button', { name: 'Sign in' }).click();
  expect((await loginResponse).status()).toBe(200);
}

test.afterEach(async () => {
  await new Promise((resolve) => setTimeout(resolve, 6500));
});

test('real pinned Cap widget solves against the local mock and creation succeeds', async ({ page, request }) => {
  const violations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' && /content security policy/i.test(message.text())) {
      violations.push(message.text());
    }
  });

  await page.goto('/');
  await page.getByRole('button', { name: 'EN' }).click();
  const textPanel = page.getByRole('tabpanel', { name: 'Text' });
  await expect(textPanel.getByRole('group', { name: 'Human verification' })).toBeVisible();
  await expect(textPanel.getByText('Verified')).toBeVisible({ timeout: 30_000 });

  // The real widget is present and solved; its server-side verifier accepted
  // the local mock's token.
  await expect(textPanel.locator('cap-widget')).toHaveCount(1);

  const config = await (await request.get('/api/v1/config')).json();
  expect(config.deployment_mode).toBe('public');
  expect(config.challenge.provider).toBe('cap');
  expect(config.challenge.site_key).toBe('test-cap-site');
  expect(JSON.stringify(config)).not.toContain('test-secret');

  await textPanel.getByRole('textbox', { name: 'Content' }).fill('cap browser content');
  const createRequest = page.waitForRequest((r) => r.method() === 'POST' && r.url().includes('/api/v1/shares'));
  const createResponse = page.waitForResponse((r) => r.request().method() === 'POST' && r.url().includes('/api/v1/shares'));
  await textPanel.getByRole('button', { name: 'Create share' }).click();

  const capturedRequest = await createRequest;
  expect(capturedRequest.headers()['x-upaste-challenge']).toBe(fakeToken);
  const response = await createResponse;
  expect(response.status()).toBe(201);
  await expect(page.getByRole('heading', { name: 'Share created' })).toBeVisible();

  const surfaces = await page.evaluate(() => JSON.stringify({
    href: location.href,
    local: localStorage,
    session: sessionStorage,
    cookie: document.cookie,
    state: history.state,
    body: document.body.textContent,
  }));
  expect(surfaces).not.toContain(fakeToken);
  expect(surfaces).not.toContain(adminToken);
  expect(capturedRequest.url()).not.toContain(fakeToken);
  expect(violations).toEqual([]);
});

test('admin inspects and deletes a Cap-created Share', async ({ page, request }) => {
  const content = 'admin governance text';
  await page.goto('/');
  await page.getByRole('button', { name: 'EN' }).click();
  const textPanel = page.getByRole('tabpanel', { name: 'Text' });
  await expect(textPanel.getByText('Verified')).toBeVisible({ timeout: 30_000 });
  await textPanel.getByRole('textbox', { name: 'Content' }).fill(content);
  const createResponse = page.waitForResponse((r) => r.request().method() === 'POST' && r.url().includes('/api/v1/shares'));
  await textPanel.getByRole('button', { name: 'Create share' }).click();
  const created = await (await createResponse).json();
  await expect(page.getByRole('heading', { name: 'Share created' })).toBeVisible();

  await page.goto('/admin');
  const listResponsePromise = page.waitForResponse((r) => {
    const url = new URL(r.url());
    return r.request().method() === 'GET' && url.pathname === '/api/v1/admin/shares' && !url.searchParams.has('id');
  });
  await signInAdmin(page);
  const listResponse = await listResponsePromise;
  const listBody = await listResponse.json();
  const listRaw = JSON.stringify(listBody);
  expect(listRaw).not.toContain(content);
  expect(listRaw).not.toContain('"text"');
  expect(listRaw).not.toContain('"encrypted_text"');
  expect(listRaw).not.toContain('"file"');
  expect(listRaw).not.toContain('"nonce"');
  expect(listRaw).not.toContain('"ciphertext"');
  const listed = listBody.shares.find((item: { id: string }) => item.id === created.share.id);
  expect(listed).toBeTruthy();
  expect(listed.payload_bytes).toBe(Buffer.byteLength(content));

  const row = page.getByRole('row').filter({ hasText: created.share.id });
  const detailRequest = page.waitForRequest((r) => r.method() === 'GET' && r.url().endsWith(`/api/v1/admin/shares/${created.share.id}`));
  await row.getByRole('button', { name: 'Inspect' }).click();
  await detailRequest;

  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText(content)).toBeVisible();
  await expect(dialog.getByText(`${Buffer.byteLength(content)} B`)).toBeVisible();
  await expect(dialog).not.toContainText('NaN');
  await expect(dialog).not.toContainText('undefined');

  // detail -> delete confirmation must replace, never stack, dialogs.
  await dialog.getByRole('button', { name: 'Delete share' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(1);
  await expect(page.getByRole('dialog')).toHaveAccessibleName('Delete this share?');
  await page.getByRole('button', { name: 'Delete permanently' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText(created.share.id)).toHaveCount(0);
  expect((await request.get(`/api/v1/shares/${created.share.id}`)).status()).toBe(404);
});

test('admin exact lookup, selection reset, and race-safe detail loading', async ({ page, request }) => {
  const contentA = 'first exact-safe share';
  const contentB = 'second exact-safe share';
  const idA = await createTextShare(request, contentA);
  const idB = await createTextShare(request, contentB);

  await page.goto('/admin');
  await signInAdmin(page);
  await expect(page.getByText(idA)).toBeVisible();
  await expect(page.getByText(idB)).toBeVisible();

  let listRequests = 0;
  page.on('request', (req) => {
    if (req.method() !== 'GET') return;
    const url = new URL(req.url());
    if (url.pathname === '/api/v1/admin/shares') listRequests++;
  });

  const lookup = page.getByLabel('Exact Share ID');
  await lookup.fill(idB.slice(0, 8));
  await page.waitForTimeout(250);
  expect(listRequests).toBe(0);

  await page.getByRole('button', { name: 'Find' }).click();
  await expect(page.getByRole('alert')).toContainText('Share IDs are 22 URL-safe characters.');
  expect(listRequests).toBe(0);

  const checkboxA = page.getByLabel(`Select ${idA}`);
  await checkboxA.check();
  await expect(checkboxA).toBeChecked();

  await lookup.fill(idA);
  const exactResponse = page.waitForResponse((r) => {
    const url = new URL(r.url());
    return r.request().method() === 'GET' && url.pathname === '/api/v1/admin/shares' && url.searchParams.get('id') === idA;
  });
  await page.getByRole('button', { name: 'Find' }).click();
  await exactResponse;
  await expect(page.getByText(idA)).toBeVisible();
  await expect(checkboxA).not.toBeChecked();

  const clearResponse = page.waitForResponse((r) => {
    const url = new URL(r.url());
    return r.request().method() === 'GET' && url.pathname === '/api/v1/admin/shares' && !url.searchParams.has('id');
  });
  await page.getByRole('button', { name: 'Clear' }).click();
  await clearResponse;
  await expect(lookup).toHaveValue('');
  await expect(page.getByText(idB)).toBeVisible();

  await page.getByLabel(`Select ${idA}`).check();
  await expect(page.getByLabel(`Select ${idA}`)).toBeChecked();
  const sortResponse = page.waitForResponse((r) => {
    const url = new URL(r.url());
    return r.request().method() === 'GET' && url.pathname === '/api/v1/admin/shares' && url.searchParams.get('sort') === 'oldest';
  });
  await page.getByLabel('Sort order').selectOption('oldest');
  await sortResponse;
  await expect(page.getByLabel(`Select ${idA}`)).not.toBeChecked();

  // Delay the first detail GET, close it while loading, then open B. The late A
  // response must neither reopen a dialog nor overwrite B.
  let delayed = false;
  await page.route('**/api/v1/admin/shares/*', async (route) => {
    if (route.request().method() === 'GET' && !delayed) {
      delayed = true;
      await new Promise((resolve) => setTimeout(resolve, 1200));
    }
    await route.continue().catch(() => {});
  });
  const rowA = page.getByRole('row').filter({ hasText: idA });
  const rowB = page.getByRole('row').filter({ hasText: idB });
  await rowA.getByRole('button', { name: 'Inspect' }).click();
  await expect(page.getByText('Loading Share detail…')).toBeVisible();
  await page.getByRole('dialog').getByRole('button', { name: 'Close' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  await rowB.getByRole('button', { name: 'Inspect' }).click();
  await expect(page.getByRole('dialog').getByText(contentB)).toBeVisible();
  await page.waitForTimeout(1500);
  await expect(page.getByRole('dialog')).toHaveCount(1);
  await expect(page.getByRole('dialog').getByText(contentB)).toBeVisible();
  await expect(page.getByText(contentA)).toHaveCount(0);
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.unroute('**/api/v1/admin/shares/*');
  await page.getByRole('dialog').getByRole('button', { name: 'Close' }).click();
});

test('admin bulk delete reports partial failures and reloads successful deletions', async ({ page, request }) => {
  const contentA = 'partial failure alpha';
  const contentB = 'partial failure bravo';
  const contentC = 'partial failure charlie';
  const idA = await createTextShare(request, contentA);
  const idB = await createTextShare(request, contentB);
  const idC = await createTextShare(request, contentC);

  await page.goto('/admin');
  await signInAdmin(page);
  await expect(page.getByText(idA)).toBeVisible();
  await expect(page.getByText(idB)).toBeVisible();
  await expect(page.getByText(idC)).toBeVisible();

  // Use a separate API admin session to remove one selected Share behind the
  // table's back, producing a real backend { deleted, failed } partial result.
  const loginResponse = await request.post('/api/v1/admin/session', { data: { token: adminToken } });
  expect(loginResponse.status()).toBe(200);
  const { csrf } = await loginResponse.json();
  const setCookie = loginResponse.headersArray().find((header) => header.name.toLowerCase() === 'set-cookie')?.value;
  expect(setCookie).toBeTruthy();
  const sessionCookie = setCookie!.split(';', 1)[0];
  const directDelete = await request.delete(`/api/v1/admin/shares/${idA}`, {
    headers: { 'X-uPaste-CSRF': csrf, Cookie: sessionCookie },
  });
  expect(directDelete.status()).toBe(204);
  await expect(page.getByText(idA)).toBeVisible();

  await page.getByLabel(`Select ${idA}`).check();
  await page.getByLabel(`Select ${idB}`).check();
  await page.getByLabel(`Select ${idC}`).check();
  const bulkResponsePromise = page.waitForResponse((r) => r.request().method() === 'POST' && r.url().includes('/api/v1/admin/shares/bulk-delete'));
  await page.getByRole('button', { name: 'Delete selected (3)', exact: true }).click();
  await page.getByRole('button', { name: 'Delete selected', exact: true }).click();

  const bulkResponse = await bulkResponsePromise;
  const bulkBody = await bulkResponse.json();
  expect(bulkBody.deleted).toBe(2);
  expect(bulkBody.failed).toEqual([idA]);

  await expect(page.getByRole('status')).toContainText('Deleted 2 shares; 1 could not be deleted.');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText(idA)).toHaveCount(0);
  await expect(page.getByText(idB)).toHaveCount(0);
  await expect(page.getByText(idC)).toHaveCount(0);
});
