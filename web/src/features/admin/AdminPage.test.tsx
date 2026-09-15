import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';
import { MemoryRouter } from 'react-router';

vi.mock('../../app/adminApi', () => ({
  AdminApiError: class AdminApiError extends Error {
    status = 0;
    code = '';
  },
  getAdminSession: vi.fn(),
  loginAdmin: vi.fn(),
  logoutAdmin: vi.fn(),
  getAdminSummary: vi.fn(),
  listAdminShares: vi.fn(),
  getAdminShare: vi.fn(),
  deleteAdminShare: vi.fn(),
  bulkDeleteAdminShares: vi.fn(),
  cleanupAdminExpired: vi.fn(),
}));

import { ConfigProvider } from '../../app/config';
import { AdminPage } from './AdminPage';
import * as adminApi from '../../app/adminApi';

const mockApi = adminApi as unknown as Record<string, ReturnType<typeof vi.fn>>;

function renderAdmin() {
  return render(
    <MemoryRouter>
      <ConfigProvider>
        <AdminPage />
      </ConfigProvider>
    </MemoryRouter>,
  );
}

describe('AdminPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    mockApi.getAdminSession.mockReset();
    mockApi.loginAdmin.mockReset();
    mockApi.getAdminSummary.mockReset();
    mockApi.listAdminShares.mockReset();
    mockApi.getAdminShare.mockReset();
    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ deployment_mode: 'public', admin_enabled: true, retention: { default_seconds: 86400, max_seconds: 604800 }, challenge: { provider: 'cap', site_key: 'site', api_endpoint: 'https://cap.example.com' } }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    ) as any;
  });

  it('shows that administration is unavailable when disabled', () => {
    render(
      <MemoryRouter>
        <AdminPage />
      </MemoryRouter>,
    );
    expect(screen.getByRole('heading', { name: 'Administration unavailable' })).toBeInTheDocument();
  });

  it('loads Share detail on demand when Inspect is clicked', async () => {
    mockApi.getAdminSession.mockResolvedValue(null);
    mockApi.loginAdmin.mockResolvedValue({ csrf: 'csrf-1' });
    mockApi.getAdminSummary.mockResolvedValue({ summary: { active: 1, expired: 0, text: 1, file: 0, encrypted: 0, file_bytes: 0 } });
    mockApi.listAdminShares.mockResolvedValue({ shares: [{ id: 'BBBBBBBBBBBBBBBBBBBBBB', payload_kind: 'TEXT', privacy_mode: 'STANDARD', state: 'active', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z', expires_at: null, payload_bytes: 12 }], next_cursor: '' });
    mockApi.getAdminShare.mockResolvedValue({ share: { id: 'BBBBBBBBBBBBBBBBBBBBBB', payload_kind: 'TEXT', privacy_mode: 'STANDARD', state: 'active', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z', expires_at: null, payload_bytes: 12, text: { format: 'PLAIN', content: 'fetched on demand' } } });

    const user = userEvent.setup();
    renderAdmin();
    await user.type(await screen.findByLabelText('Admin token'), 'up_a1_test');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));
    await waitFor(() => expect(screen.getByText('BBBBBBBBBBBBBBBBBBBBBB')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Inspect' }));
    await waitFor(() => expect(mockApi.getAdminShare).toHaveBeenCalledWith('BBBBBBBBBBBBBBBBBBBBBB'));
    await waitFor(() => expect(screen.getByText('fetched on demand')).toBeInTheDocument());
  });

  it('authenticates, lists Shares, and logs out', async () => {
    mockApi.getAdminSession.mockResolvedValue(null);
    mockApi.loginAdmin.mockResolvedValue({ csrf: 'csrf-1' });
    mockApi.getAdminSummary.mockResolvedValue({ summary: { active: 1, expired: 0, text: 1, file: 0, encrypted: 0, file_bytes: 0 } });
    mockApi.listAdminShares.mockResolvedValue({ shares: [{ id: 'AAAAAAAAAAAAAAAAAAAAAA', payload_kind: 'TEXT', privacy_mode: 'STANDARD', state: 'active', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z', expires_at: null, payload_bytes: 14 }], next_cursor: '' });
    mockApi.logoutAdmin.mockResolvedValue(undefined);

    const user = userEvent.setup();
    renderAdmin();
    const input = await screen.findByLabelText('Admin token');
    await user.type(input, 'up_a1_test');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));
    await waitFor(() => expect(screen.getByText('AAAAAAAAAAAAAAAAAAAAAA')).toBeInTheDocument());
    expect(mockApi.loginAdmin).toHaveBeenCalledWith('up_a1_test');
    await user.click(screen.getByRole('button', { name: 'Log out' }));
    await waitFor(() => expect(screen.getByLabelText('Admin token')).toBeInTheDocument());
  });
});
