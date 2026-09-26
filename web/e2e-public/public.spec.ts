import { expect, test, type Page } from '@playwright/test';

const turnstileScript = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
const fakeToken = 'test-challenge-token';
const adminToken = `up_a1_${'A'.repeat(43)}`;

test.afterEach(async () => {
  await new Promise((resolve) => setTimeout(resolve, 6500));
});

async function installFakeTurnstile(page: Page): Promise<string[]> {
  const violations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' && /content security policy/i.test(message.text())) {
      violations.push(message.text());
    }
  });
  await page.route(turnstileScript, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/javascript',
      body: `window.turnstile = { render: function(element, options) { options.callback('${fakeToken}'); return 'test-widget'; }, remove: function() {}, reset: function() {} };`,
    }),
  );
  return violations;
}

test('public mode requires a challenge token in the dedicated header', async ({ page, request }) => {
  const violations = await installFakeTurnstile(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'EN' }).click();
  const textPanel = page.getByRole('tabpanel', { name: 'Text' });
  await expect(textPanel.getByText('Verified')).toBeVisible({ timeout: 15_000 });

  // Config endpoint exposes policy but no secrets.
  const config = await (await request.get('/api/v1/config')).json();
  expect(config.deployment_mode).toBe('public');
  expect(config.retention.default_seconds).toBe(86400);
  expect(config.retention.max_seconds).toBe(604800);
  expect(config.challenge.provider).toBe('turnstile');
  expect(config.challenge.site_key).toBe('1x00000000000000000000AA');
  expect(JSON.stringify(config)).not.toContain('test-secret');

  await textPanel.getByRole('textbox', { name: 'Content' }).fill('public challenge text');
  const createRequest = page.waitForRequest((r) => r.method() === 'POST' && r.url().includes('/api/v1/shares'));
  const createResponse = page.waitForResponse((r) => r.request().method() === 'POST' && r.url().includes('/api/v1/shares'));
  await textPanel.getByRole('button', { name: 'Create share' }).click();

  const requestHeaders = (await createRequest).headers();
  expect(requestHeaders['x-upaste-challenge']).toBe(fakeToken);
  const response = await createResponse;
  expect([403, 503]).toContain(response.status());
  const body = await response.text();
  expect(body).toMatch(/challenge_(failed|unavailable)/);
  expect(body).not.toContain(fakeToken);

  // The token is not persisted in browser surfaces.
  const surfaces = await page.evaluate(() => JSON.stringify({
    local: localStorage,
    session: sessionStorage,
    cookie: document.cookie,
    state: history.state,
    body: document.body.textContent,
  }));
  expect(surfaces).not.toContain(fakeToken);
  expect(surfaces).not.toContain(adminToken);
  expect(violations).toEqual([]);
});

test('public mode hides Never and 30 days expiration choices', async ({ page }) => {
  await page.route(turnstileScript, (route) =>
    route.fulfill({ status: 200, contentType: 'application/javascript', body: `window.turnstile = { render: function(element, options) { return 'w'; }, remove: function() {}, reset: function() {} };` }),
  );
  await page.goto('/');
  await page.getByRole('button', { name: 'EN' }).click();
  const expiration = page.getByRole('tabpanel', { name: 'Text' }).getByLabel('Expiration');
  await expect(expiration).toHaveValue('1d');
  const optionTexts = await expiration.locator('option').allTextContents();
  expect(optionTexts).not.toContain('Never');
  expect(optionTexts).not.toContain('30 days');
  await expect(page.getByRole('group', { name: 'Human verification' })).toBeVisible();
});

test('admin login lists governance data without exposing the admin token', async ({ page }) => {
  await page.goto('/admin');
  const tokenInput = page.getByLabel('Admin token');
  await expect(tokenInput).toBeVisible();
  await tokenInput.fill(adminToken);
  const loginResponse = page.waitForResponse((r) => r.request().method() === 'POST' && r.url().includes('/api/v1/admin/session'));
  await page.getByRole('button', { name: 'Sign in' }).click();
  expect((await loginResponse).status()).toBe(200);
  await expect(page.getByRole('heading', { name: 'uPaste / Administration' })).toBeVisible();
  await expect(page.getByText(/Stored files:/)).toBeVisible();

  const surfaces = await page.evaluate(() => JSON.stringify({
    href: location.href,
    state: history.state,
    local: localStorage,
    session: sessionStorage,
    body: document.body.textContent,
  }));
  expect(surfaces).not.toContain(adminToken);

  await page.getByRole('button', { name: 'Log out' }).click();
  await expect(page.getByLabel('Admin token')).toBeVisible();
});
