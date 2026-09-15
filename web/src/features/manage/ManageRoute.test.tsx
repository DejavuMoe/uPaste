import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { ThemeProvider } from '../../app/theme';
import { OwnerCapabilityProvider, useOwnerCapabilities } from '../../app/ownerCapabilities';
import { ManageRoute } from './ManageRoute';
import * as api from '../../app/api';

const encrypted = { id: 'share', payload_kind: 'TEXT' as const, privacy_mode: 'ENCRYPTED' as const, created_at: '', updated_at: '', expires_at: null, encrypted_text: { protocol: 'UPASTE_AES_GCM_V1', nonce: 'x', ciphertext: 'x' } };
function Page({ token }: { token?: string }) { const caps = useOwnerCapabilities(); if (token) caps.remember('share', token); return <RouterProvider router={createMemoryRouter([{ path: '/manage/:id', element: <ManageRoute /> }], { initialEntries: ['/manage/share'] })} />; }
function renderManage(token = 'up_o1_test') { return render(<ThemeProvider><OwnerCapabilityProvider><Page token={token} /></OwnerCapabilityProvider></ThemeProvider>); }

describe('ManageRoute focused regressions', () => {
  it('reaches ready no-key encrypted management', async () => {
    vi.spyOn(api, 'getShare').mockResolvedValue({ share: encrypted });
    renderManage();
    await waitFor(() => expect(screen.getByText(/Content editing is unavailable/)).toBeInTheDocument());
    expect(screen.queryByLabelText('Editor')).toBeNull();
    expect(screen.getByRole('button', { name: 'Delete share' })).toBeInTheDocument();
  });
});
