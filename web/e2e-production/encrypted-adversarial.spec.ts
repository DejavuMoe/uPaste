import { expect, test, type Page } from '@playwright/test';

test.afterEach(async () => {
  await new Promise((resolve) => setTimeout(resolve, 6500));
});

interface EncryptedShare {
  id: string;
  token: string;
  fragment: string;
  content: string;
  payload: any;
}

async function createEncrypted(page: Page, format: 'PLAIN' | 'MARKDOWN', content: string): Promise<EncryptedShare> {
  await page.goto('/');
  if (format === 'MARKDOWN') {
    await page.getByRole('radio', { name: 'Markdown', exact: true }).click();
  }
  await page.getByRole('radio', { name: 'Encrypted', exact: true }).click();
  await page.getByRole('textbox', { name: 'Share text content' }).fill(content);
  const response = page.waitForResponse(
    (r) => r.request().method() === 'POST' && r.url().includes('/api/v1/shares'),
  );
  await page.getByRole('button', { name: 'Create share' }).click();
  const payload = await (await response).json();
  const link = await page.getByLabel('Share link').inputValue();
  return {
    id: payload.share.id as string,
    token: payload.owner_token as string,
    fragment: new URL(link).hash,
    content,
    payload,
  };
}

test('encrypted browser rejects missing keys, wrong keys, and tampered ciphertext', async ({ page, request }) => {
  const content = 'encrypted adversarial plaintext';
  const share = await createEncrypted(page, 'PLAIN', content);

  // The plaintext never reached storage or API responses.
  const apiBody = await (await request.get(`/api/v1/shares/${share.id}`)).text();
  expect(apiBody).not.toContain(content);
  expect(apiBody).not.toContain(share.fragment);
  expect(share.fragment).toMatch(/^#up_e1_[A-Za-z0-9_-]{43}$/);

  const seenUrls: string[] = [];
  page.on('request', (request) => seenUrls.push(request.url()));

  // Missing fragment.
  await page.goto(`/s/${share.id}`);
  await expect(page.getByRole('heading', { name: 'Decryption key missing' })).toBeVisible();

  // Malformed fragment.
  await page.goto(`/s/${share.id}#not-a-key`);
  await expect(page.getByRole('heading', { name: 'Unable to decrypt this share' })).toBeVisible();

  // Valid-looking but wrong key.
  const wrongFragment = `#up_e1_${'A'.repeat(43)}`;
  await page.goto(`/s/${share.id}${wrongFragment}`);
  await expect(page.getByRole('heading', { name: 'Unable to decrypt this share' })).toBeVisible();

  // Tampered nonce: server accepts a fresh valid nonce, browser decrypt fails.
  const nonce = share.payload.share.encrypted_text.nonce as string;
  const ciphertext = share.payload.share.encrypted_text.ciphertext as string;
  const replacementNonce = nonce === 'BBBBBBBBBBBBBBBB' ? 'CCCCCCCCCCCCCCCC' : 'BBBBBBBBBBBBBBBB';
  let patch = await request.patch(`/api/v1/shares/${share.id}`, {
    headers: { Authorization: `Bearer ${share.token}`, 'Content-Type': 'application/json' },
    data: { encrypted_text: { protocol: 'UPASTE_AES_GCM_V1', nonce: replacementNonce, ciphertext } },
  });
  expect(patch.status()).toBe(200);
  await page.goto(`/s/${share.id}${share.fragment}`);
  await expect(page.getByRole('heading', { name: 'Unable to decrypt this share' })).toBeVisible();

  // Tampered ciphertext with a canonical replacement character.
  const tampered = (ciphertext[0] === 'A' ? 'B' : 'A') + ciphertext.slice(1);
  patch = await request.patch(`/api/v1/shares/${share.id}`, {
    headers: { Authorization: `Bearer ${share.token}`, 'Content-Type': 'application/json' },
    data: { encrypted_text: { protocol: 'UPASTE_AES_GCM_V1', nonce: replacementNonce, ciphertext: tampered } },
  });
  expect(patch.status()).toBe(200);
  await page.goto(`/s/${share.id}${share.fragment}`);
  await expect(page.getByRole('heading', { name: 'Unable to decrypt this share' })).toBeVisible();

  // Secret key and owner token never appear in browser request URLs.
  const serialized = seenUrls.join('\n');
  expect(serialized).not.toContain(share.fragment);
  expect(serialized).not.toContain(share.fragment.slice(7));
  expect(serialized).not.toContain(share.token);
});

test('encrypted Markdown renders only after browser decryption', async ({ page }) => {
  const markdown = '# Encrypted Markdown\n\nrendered after decryption';
  const share = await createEncrypted(page, 'MARKDOWN', markdown);
  await page.getByRole('button', { name: 'Open share' }).click();
  await expect(page.getByRole('heading', { level: 1, name: 'Encrypted Markdown' })).toBeVisible();
  await expect(page.getByText('rendered after decryption')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Raw' })).toHaveCount(0);
});
