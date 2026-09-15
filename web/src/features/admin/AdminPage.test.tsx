import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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
import type { AdminListItem, AdminShareDetail } from '../../app/adminApi';
import * as adminApi from '../../app/adminApi';

const mockApi = adminApi as unknown as Record<string, ReturnType<typeof vi.fn>>;
const ID_A = 'A'.repeat(22);
const ID_B = 'B'.repeat(22);
const ID_C = 'C'.repeat(22);
const summary = { active: 3, expired: 0, text: 3, file: 0, encrypted: 0, file_bytes: 0 };

type User = ReturnType<typeof userEvent.setup>;

function renderAdmin() {
  return render(
    <MemoryRouter>
      <ConfigProvider>
        <AdminPage />
      </ConfigProvider>
    </MemoryRouter>,
  );
}

function makeShare(id: string, payloadBytes = 12): AdminListItem {
  return {
    id,
    payload_kind: 'TEXT',
    privacy_mode: 'STANDARD',
    state: 'active',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    expires_at: null,
    payload_bytes: payloadBytes,
  };
}

function makeDetail(id: string, payloadBytes = 12, content = `detail-${id}`): AdminShareDetail {
  return { ...makeShare(id, payloadBytes), text: { format: 'PLAIN', content } };
}

function mockSessionAndSummary() {
  mockApi.getAdminSession.mockResolvedValue(null);
  mockApi.loginAdmin.mockResolvedValue({ csrf: 'csrf-1' });
  mockApi.getAdminSummary.mockResolvedValue({ summary });
}

async function signIn(user: User) {
  await user.type(await screen.findByLabelText('Admin token'), 'up_a1_test');
  await user.click(screen.getByRole('button', { name: 'Sign in' }));
}

describe('AdminPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    for (const name of [
      'getAdminSession',
      'loginAdmin',
      'logoutAdmin',
      'getAdminSummary',
      'listAdminShares',
      'getAdminShare',
      'deleteAdminShare',
      'bulkDeleteAdminShares',
      'cleanupAdminExpired',
    ]) {
      mockApi[name].mockReset();
    }
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

  it('loads Share detail on demand with a separate request', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares.mockResolvedValue({ shares: [makeShare(ID_A)], next_cursor: '' });
    mockApi.getAdminShare.mockResolvedValue({ share: makeDetail(ID_A, 12, 'fetched on demand') });

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await waitFor(() => expect(screen.getByText(ID_A)).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Inspect' }));
    await waitFor(() => expect(mockApi.getAdminShare).toHaveBeenCalledWith(ID_A, expect.any(AbortSignal)));
    expect(mockApi.listAdminShares).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.getByText('fetched on demand')).toBeInTheDocument());
  });

  it('never renders NaN or undefined for an absent size value', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares.mockResolvedValue({ shares: [makeShare(ID_A)], next_cursor: '' });
    mockApi.getAdminShare.mockResolvedValue({ share: { ...makeDetail(ID_A), payload_bytes: undefined as unknown as number } });

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await waitFor(() => expect(screen.getByText(ID_A)).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Inspect' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('0 B')).toBeInTheDocument();
    expect(dialog).not.toHaveTextContent('NaN');
    expect(dialog).not.toHaveTextContent('undefined');
  });

  it('uses exactly one dialog from detail to delete, restores detail on Escape, and removes stale dialogs', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares
      .mockResolvedValueOnce({ shares: [makeShare(ID_A)], next_cursor: '' })
      .mockResolvedValueOnce({ shares: [], next_cursor: '' });
    mockApi.getAdminShare.mockResolvedValue({ share: makeDetail(ID_A, 12, 'delete me') });
    mockApi.deleteAdminShare.mockResolvedValue(undefined);

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await user.click(await screen.findByRole('button', { name: 'Inspect' }));
    await screen.findByText('delete me');

    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    await user.click(screen.getByRole('button', { name: 'Delete share' }));
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    expect(screen.getByRole('dialog')).toHaveAccessibleName('Delete this share?');

    await user.keyboard('{Escape}');
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    expect(screen.getByRole('dialog')).toHaveAccessibleName('Share detail');
    expect(screen.getByText('delete me')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Delete share' }));
    await user.click(screen.getByRole('button', { name: 'Delete permanently' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await waitFor(() => expect(screen.queryByText(ID_A)).not.toBeInTheDocument());
    expect(mockApi.deleteAdminShare).toHaveBeenCalledWith(ID_A, 'csrf-1');
  });

  it('preserves selection on Load more and clears it when the base query changes', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares
      .mockResolvedValueOnce({ shares: [makeShare(ID_A), makeShare(ID_B)], next_cursor: 'cursor-1' })
      .mockResolvedValueOnce({ shares: [makeShare(ID_C)], next_cursor: '' })
      .mockResolvedValueOnce({ shares: [makeShare(ID_A), makeShare(ID_B), makeShare(ID_C)], next_cursor: '' });

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await waitFor(() => expect(screen.getByText(ID_A)).toBeInTheDocument());

    const checkboxA = screen.getByLabelText(`Select ${ID_A}`);
    await user.click(checkboxA);
    expect(checkboxA).toBeChecked();

    await user.click(screen.getByRole('button', { name: 'Load more' }));
    await waitFor(() => expect(screen.getByText(ID_C)).toBeInTheDocument());
    expect(screen.getByLabelText(`Select ${ID_A}`)).toBeChecked();

    await user.selectOptions(screen.getByLabelText('Sort order'), 'oldest');
    await waitFor(() => expect(screen.getByLabelText(`Select ${ID_A}`)).not.toBeChecked());
    expect(mockApi.listAdminShares).toHaveBeenLastCalledWith(expect.objectContaining({ sort: 'oldest' }));
  });

  it('keeps exact-ID drafting local and only applies a valid submitted ID', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares.mockResolvedValue({ shares: [makeShare(ID_A), makeShare(ID_B)], next_cursor: '' });

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await waitFor(() => expect(screen.getByText(ID_A)).toBeInTheDocument());
    const listCallsAfterLoad = mockApi.listAdminShares.mock.calls.length;

    const lookup = screen.getByLabelText('Exact Share ID');
    await user.type(lookup, ID_A.slice(0, 8));
    await waitFor(() => expect(lookup).toHaveValue(ID_A.slice(0, 8)));
    expect(mockApi.listAdminShares).toHaveBeenCalledTimes(listCallsAfterLoad);

    await user.click(screen.getByRole('button', { name: 'Find' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Share IDs are 22 URL-safe characters.');
    expect(mockApi.listAdminShares).toHaveBeenCalledTimes(listCallsAfterLoad);

    const checkboxA = screen.getByLabelText(`Select ${ID_A}`);
    await user.click(checkboxA);
    expect(checkboxA).toBeChecked();

    await user.clear(lookup);
    await user.type(lookup, ID_A);
    await user.click(screen.getByRole('button', { name: 'Find' }));
    await waitFor(() => expect(mockApi.listAdminShares).toHaveBeenLastCalledWith(expect.objectContaining({ id: ID_A })));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByLabelText(`Select ${ID_A}`)).not.toBeChecked());

    await user.click(screen.getByRole('button', { name: 'Clear' }));
    await waitFor(() => expect(screen.getByLabelText('Exact Share ID')).toHaveValue(''));
    const lastParams = mockApi.listAdminShares.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(lastParams.id).toBeUndefined();
  });

  it('ignores a late detail response after close and never lets A overwrite B', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares.mockResolvedValue({ shares: [makeShare(ID_A), makeShare(ID_B)], next_cursor: '' });
    const pending = new Map<string, (value: { share: AdminShareDetail }) => void>();
    mockApi.getAdminShare.mockImplementation((id: string) => new Promise((resolve) => pending.set(id, resolve)));

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await waitFor(() => expect(screen.getByText(ID_A)).toBeInTheDocument());

    await user.click(screen.getAllByRole('button', { name: 'Inspect' })[0]);
    await screen.findByText('Loading Share detail…');
    const signalA = mockApi.getAdminShare.mock.calls[0][1] as AbortSignal;
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Close' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(signalA.aborted).toBe(true);

    await user.click(screen.getAllByRole('button', { name: 'Inspect' })[1]);
    await waitFor(() => expect(mockApi.getAdminShare).toHaveBeenCalledWith(ID_B, expect.any(AbortSignal)));
    pending.get(ID_B)!({ share: makeDetail(ID_B, 12, 'detail B') });
    await screen.findByText('detail B');

    pending.get(ID_A)!({ share: makeDetail(ID_A, 12, 'detail A') });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    expect(screen.getByText('detail B')).toBeInTheDocument();
    expect(screen.queryByText('detail A')).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('aborts detail loading on logout and does not surface an intentional abort', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares.mockResolvedValue({ shares: [makeShare(ID_A)], next_cursor: '' });
    const pending: Array<(value: { share: AdminShareDetail }) => void> = [];
    mockApi.getAdminShare.mockImplementation(() => new Promise((resolve) => pending.push(resolve)));
    mockApi.logoutAdmin.mockResolvedValue(undefined);

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await user.click(await screen.findByRole('button', { name: 'Inspect' }));
    await screen.findByText('Loading Share detail…');
    const signal = mockApi.getAdminShare.mock.calls[0][1] as AbortSignal;

    fireEvent.click(screen.getByRole('button', { name: 'Log out' }));
    await waitFor(() => expect(screen.getByLabelText('Admin token')).toBeInTheDocument());
    expect(signal.aborted).toBe(true);
    pending[0]({ share: makeDetail(ID_A, 12, 'late after logout') });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('aborts detail loading when the page unmounts', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares.mockResolvedValue({ shares: [makeShare(ID_A)], next_cursor: '' });
    const pending: Array<(value: { share: AdminShareDetail }) => void> = [];
    mockApi.getAdminShare.mockImplementation(() => new Promise((resolve) => pending.push(resolve)));

    const user = userEvent.setup();
    const { unmount } = renderAdmin();
    await signIn(user);
    await user.click(await screen.findByRole('button', { name: 'Inspect' }));
    await screen.findByText('Loading Share detail…');
    const signal = mockApi.getAdminShare.mock.calls[0][1] as AbortSignal;

    unmount();
    expect(signal.aborted).toBe(true);
    pending[0]({ share: makeDetail(ID_A, 12, 'late after unmount') });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });

  it('clears selection and closes normally after a fully successful bulk delete', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares
      .mockResolvedValueOnce({ shares: [makeShare(ID_A), makeShare(ID_B)], next_cursor: '' })
      .mockResolvedValueOnce({ shares: [], next_cursor: '' });
    mockApi.bulkDeleteAdminShares.mockResolvedValue({ deleted: 2, failed: [] });

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await user.click(await screen.findByLabelText(`Select ${ID_A}`));
    await user.click(screen.getByLabelText(`Select ${ID_B}`));
    await user.click(screen.getByRole('button', { name: 'Delete selected (2)' }));
    await user.click(screen.getByRole('button', { name: 'Delete selected' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(screen.getByRole('status')).toHaveTextContent('Deleted 2 shares.');
    expect(screen.queryByText(ID_A)).not.toBeInTheDocument();
    expect(screen.queryByText(ID_B)).not.toBeInTheDocument();
    expect(mockApi.bulkDeleteAdminShares).toHaveBeenCalledWith([ID_A, ID_B], 'csrf-1');
  });

  it('reports partial bulk failure and reselects failed IDs that remain visible', async () => {
    mockSessionAndSummary();
    mockApi.listAdminShares
      .mockResolvedValueOnce({ shares: [makeShare(ID_A), makeShare(ID_B), makeShare(ID_C)], next_cursor: '' })
      .mockResolvedValueOnce({ shares: [makeShare(ID_B)], next_cursor: '' });
    mockApi.bulkDeleteAdminShares.mockResolvedValue({ deleted: 2, failed: [ID_B] });

    const user = userEvent.setup();
    renderAdmin();
    await signIn(user);
    await user.click(await screen.findByLabelText(`Select ${ID_A}`));
    await user.click(screen.getByLabelText(`Select ${ID_B}`));
    await user.click(screen.getByLabelText(`Select ${ID_C}`));
    await user.click(screen.getByRole('button', { name: 'Delete selected (3)' }));
    await user.click(screen.getByRole('button', { name: 'Delete selected' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(await screen.findByRole('status')).toHaveTextContent('Deleted 2 shares; 1 could not be deleted.');
    await waitFor(() => expect(screen.queryByText(ID_A)).not.toBeInTheDocument());
    expect(screen.queryByText(ID_C)).not.toBeInTheDocument();
    expect(screen.getByText(ID_B)).toBeInTheDocument();
    expect(screen.getByLabelText(`Select ${ID_B}`)).toBeChecked();
    expect(mockApi.bulkDeleteAdminShares).toHaveBeenCalledWith([ID_A, ID_B, ID_C], 'csrf-1');
  });
});
