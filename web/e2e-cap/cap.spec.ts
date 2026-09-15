import { expect, test } from '@playwright/test';

const fakeToken = 'cap-test-token';
const adminToken = `up_a1_${'A'.repeat(43)}`;

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

  await textPanel.getByRole('textbox', { name: 'Share text content' }).fill('cap browser content');
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
  await page.goto('/');
  const textPanel = page.getByRole('tabpanel', { name: 'Text' });
  await expect(textPanel.getByText('Verified')).toBeVisible({ timeout: 30_000 });
  await textPanel.getByRole('textbox', { name: 'Share text content' }).fill('admin governance text');
  const createResponse = page.waitForResponse((r) => r.request().method() === 'POST' && r.url().includes('/api/v1/shares'));
  await textPanel.getByRole('button', { name: 'Create share' }).click();
  const created = await (await createResponse).json();
  await expect(page.getByRole('heading', { name: 'Share created' })).toBeVisible();

  await page.goto('/admin');
  await page.getByLabel('Admin token').fill(adminToken);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByText(created.share.id)).toBeVisible();

  await page.getByRole('button', { name: 'Inspect' }).first().click();
  await expect(page.getByText('admin governance text')).toBeVisible();
  await page.getByRole('button', { name: 'Delete share' }).click();
  await page.getByRole('button', { name: 'Delete permanently' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText(created.share.id)).toHaveCount(0);
  expect((await request.get(`/api/v1/shares/${created.share.id}`)).status()).toBe(404);
});
