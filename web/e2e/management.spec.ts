import { expect, test } from '@playwright/test';

test('standard text can be managed without leaking its owner token', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('textbox', { name: 'Share text content' }).fill('e2e original');
  await page.getByRole('button', { name: 'Create share' }).click();
  await expect(page.getByText('Share created')).toBeVisible();
  await page.getByRole('button', { name: 'Reveal management token' }).click();
  const token = await page.getByRole('textbox', { name: 'Management token', exact: true }).inputValue();
  await page.getByRole('button', { name: 'Manage share' }).click();
  await expect(page.getByLabel('Editor')).toHaveValue('e2e original');
  expect(page.url()).not.toContain(token);
  await expect.poll(() => page.evaluate(() => JSON.stringify({ localStorage, sessionStorage, state: history.state, hash: location.hash }))).not.toContain(token);
  await page.getByLabel('Editor').fill('e2e updated');
  await page.getByRole('button', { name: 'Save changes' }).click();
  await expect(page.getByText('Changes saved')).toBeVisible();
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(page.getByText('e2e updated')).toBeVisible();
});

test('delete uses a dialog and removes the share', async ({ page, request }) => {
  await page.goto('/');
  await page.getByRole('textbox', { name: 'Share text content' }).fill('delete me');
  await page.getByRole('button', { name: 'Create share' }).click();
  const url = await page.getByLabel('Share link').inputValue();
  await page.getByRole('button', { name: 'Manage share' }).click();
  await page.getByRole('button', { name: 'Delete share' }).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await page.getByRole('button', { name: 'Cancel' }).last().click();
  await page.getByRole('button', { name: 'Delete share' }).click();
  await page.getByRole('button', { name: 'Delete permanently' }).click();
  await expect(page.getByText('Share deleted')).toBeVisible();
  const id = new URL(url).pathname.split('/').pop();
  expect((await request.get(`http://127.0.0.1:4180/api/v1/shares/${id}`)).status()).toBe(404);
});
