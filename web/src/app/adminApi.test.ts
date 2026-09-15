import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  bulkDeleteAdminShares,
  cleanupAdminExpired,
  deleteAdminShare,
  getAdminSession,
  loginAdmin,
  logoutAdmin,
} from './adminApi';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('adminApi', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('logs in with the admin token in the JSON body only', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ authenticated: true, csrf: 'csrf-value', expires_at: '2026-01-01T00:00:00Z' }));
    globalThis.fetch = fetchMock as any;
    await expect(loginAdmin('up_a1_secret')).resolves.toMatchObject({ csrf: 'csrf-value' });
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/admin/session');
    expect(options.method).toBe('POST');
    expect(options.credentials).toBe('same-origin');
    expect(options.body).toContain('up_a1_secret');
    expect(String(url)).not.toContain('up_a1_secret');
  });

  it('returns null for an unauthenticated session', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(jsonResponse({ error: { code: 'unauthorized', message: 'x' } }, 401)) as any;
    await expect(getAdminSession()).resolves.toBeNull();
  });

  it('sends CSRF on destructive admin actions and never in the URL', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    globalThis.fetch = fetchMock as any;
    await logoutAdmin('csrf-1');
    await deleteAdminShare('AAAAAAAAAAAAAAAAAAAAAA', 'csrf-2');
    await bulkDeleteAdminShares(['AAAAAAAAAAAAAAAAAAAAAA'], 'csrf-3');
    await cleanupAdminExpired('csrf-4');
    for (const call of fetchMock.mock.calls) {
      const [url, options] = call;
      expect(String(url)).not.toContain('csrf');
      const headers = options?.headers ?? {};
      expect(headers['X-uPaste-CSRF']).toMatch(/^csrf-/);
    }
  });
});
