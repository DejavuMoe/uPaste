import { expect, test } from '@playwright/test';

async function createStandard(page: import('@playwright/test').Page, content = 'e2e original', expiration?: string | null) {
  await page.goto('/');
  const created = page.waitForResponse((r) => r.url().includes('/api/v1/shares') && r.request().method() === 'POST');
  await page.getByRole('textbox', { name: 'Share text content' }).fill(content);
  await page.getByRole('button', { name: 'Create share' }).click();
  const payload = await (await created).json(); await expect(page.getByText('Share created')).toBeVisible();
  return { id: payload.share.id as string, token: payload.owner_token as string };
}
async function manage(page: import('@playwright/test').Page) { await page.getByRole('button', { name: 'Manage share' }).click(); await expect(page.getByLabel('Editor')).toBeVisible(); }
async function secretSurfaces(page: import('@playwright/test').Page) { return page.evaluate(() => ({ href: location.href, hash: location.hash, state: JSON.stringify(history.state), local: JSON.stringify(localStorage), session: JSON.stringify(sessionStorage), cookie: document.cookie, body: document.body.textContent || '' })); }

test('live SPA management keeps token only in memory', async ({ page }) => { const { token } = await createStandard(page); await manage(page); expect(Object.values(await secretSurfaces(page)).join('')).not.toContain(token); });
test('reload loses capability and manual token restores management', async ({ page }) => { const { token } = await createStandard(page); await manage(page); await page.reload(); await expect(page.getByRole('heading', { name: 'Management token' })).toBeVisible(); await page.getByLabel('Management token').fill(token); await page.getByRole('button', { name: 'Continue' }).click(); await expect(page.getByLabel('Editor')).toHaveValue('e2e original'); });
test('direct management URL needs token', async ({ page }) => { const { id } = await createStandard(page); await page.goto(`/manage/${id}`); await expect(page.getByRole('heading', { name: 'Management token' })).toBeVisible(); });
test('content-only PATCH updates viewer without exposing token', async ({ page }) => { const { token } = await createStandard(page); await manage(page); const request = page.waitForRequest((r) => r.url().includes('/api/v1/shares/') && r.method() === 'PATCH'); await page.getByLabel('Editor').fill('e2e updated'); await page.getByRole('button', { name: 'Save changes' }).click(); const patch = await request, body = patch.postData() || ''; expect(body).toContain('e2e updated'); expect(body).not.toContain('expires_at'); expect(body).not.toContain(token); expect(patch.url()).not.toContain(token); expect(patch.headerValue('authorization')).toBeTruthy(); await page.getByRole('button', { name: 'Cancel' }).click(); await expect(page.getByText('e2e updated')).toBeVisible(); });
test('expiration-only PATCH omits text', async ({ page }) => { await createStandard(page); await manage(page); const request = page.waitForRequest((r) => r.url().includes('/api/v1/shares/') && r.method() === 'PATCH'); await page.getByLabel('Expiration').selectOption('1d'); await page.getByRole('button', { name: 'Save changes' }).click(); const body = JSON.parse((await request).postData()!); expect(body).toHaveProperty('expires_at'); expect(body).not.toHaveProperty('text'); });
