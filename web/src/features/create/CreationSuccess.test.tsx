import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { MemoryRouter, Routes, Route } from 'react-router';
import { CreationSuccess } from './CreationSuccess';
import { OwnerCapabilityProvider, useOwnerCapabilities } from '../../app/ownerCapabilities';

describe('CreationSuccess', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
    sessionStorage.clear();
  });

  const renderWithProviders = (ui: React.ReactElement) => {
    return render(
      <MemoryRouter>
        <OwnerCapabilityProvider>{ui}</OwnerCapabilityProvider>
      </MemoryRouter>,
    );
  };

  it('renders standard share link and masked owner token', () => {
    renderWithProviders(
      <CreationSuccess
        shareId="share123"
        ownerToken="up_o1_abcdef123456"
        onReset={vi.fn()}
      />,
    );

    expect(screen.getByLabelText(/share link/i)).toHaveValue('http://localhost:3000/s/share123');

    const tokenInput = screen.getByRole('textbox', { name: 'Management token' });
    // Token is masked by default
    expect(tokenInput).toHaveValue('up_o1_••••••••••••');
    expect(tokenInput).not.toHaveValue('up_o1_abcdef123456');

    // Warning message present
    expect(screen.getByText(/Save this token now/i)).toBeInTheDocument();
  });

  it('renders encrypted share link with #up_e1_ fragment', () => {
    renderWithProviders(
      <CreationSuccess
        shareId="encshare456"
        ownerToken="up_o1_token999"
        encryptionKey="my_secret_key_base64"
        onReset={vi.fn()}
      />,
    );

    const shareInput = screen.getByLabelText(/share link/i);
    expect(shareInput).toHaveValue(
      'http://localhost:3000/s/encshare456#up_e1_my_secret_key_base64',
    );
  });

  it('toggles token masking when clicking Reveal/Hide', () => {
    renderWithProviders(
      <CreationSuccess
        shareId="share123"
        ownerToken="up_o1_abcdef123456"
        onReset={vi.fn()}
      />,
    );

    const revealBtn = screen.getByRole('button', { name: /reveal management token/i });
    fireEvent.click(revealBtn);

    const tokenInput = screen.getByRole('textbox', { name: 'Management token' });
    expect(tokenInput).toHaveValue('up_o1_abcdef123456');

    const hideBtn = screen.getByRole('button', { name: /hide management token/i });
    fireEvent.click(hideBtn);

    expect(tokenInput).toHaveValue('up_o1_••••••••••••');
  });

  it('copies share link and owner token to clipboard', async () => {
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: {
        writeText: writeTextMock,
      },
    });

    renderWithProviders(
      <CreationSuccess
        shareId="share123"
        ownerToken="up_o1_abcdef123456"
        onReset={vi.fn()}
      />,
    );

    const copyLinkBtn = screen.getByRole('button', { name: /copy link to clipboard/i });
    fireEvent.click(copyLinkBtn);
    expect(writeTextMock).toHaveBeenCalledWith('http://localhost:3000/s/share123');

    const copyTokenBtn = screen.getByRole('button', { name: /copy token to clipboard/i });
    fireEvent.click(copyTokenBtn);
    expect(writeTextMock).toHaveBeenCalledWith('up_o1_abcdef123456');
  });

  it('navigates to manage and remembers capability in volatile memory only without leaking to history state or storage', () => {
    let capturedCapability: string | undefined = undefined;

    const pushStateSpy = vi.spyOn(window.history, 'pushState');
    const replaceStateSpy = vi.spyOn(window.history, 'replaceState');

    const TestManageRoute = () => {
      const { get } = useOwnerCapabilities();
      capturedCapability = get('share123');
      return <div>Manage Target</div>;
    };

    render(
      <MemoryRouter initialEntries={['/']}>
        <OwnerCapabilityProvider>
          <Routes>
            <Route
              path="/"
              element={
                <CreationSuccess
                  shareId="share123"
                  ownerToken="up_o1_abcdef123456"
                  onReset={vi.fn()}
                />
              }
            />
            <Route path="/manage/:id" element={<TestManageRoute />} />
          </Routes>
        </OwnerCapabilityProvider>
      </MemoryRouter>,
    );

    const manageBtn = screen.getByRole('button', { name: /manage share/i });
    fireEvent.click(manageBtn);

    expect(screen.getByText('Manage Target')).toBeInTheDocument();
    expect(capturedCapability).toBe('up_o1_abcdef123456');

    // Verify token was NOT passed in history.pushState or replaceState arguments
    for (const call of pushStateSpy.mock.calls) {
      const serialized = JSON.stringify(call);
      expect(serialized).not.toContain('up_o1_abcdef123456');
    }
    for (const call of replaceStateSpy.mock.calls) {
      const serialized = JSON.stringify(call);
      expect(serialized).not.toContain('up_o1_abcdef123456');
    }

    // Verify token was NOT stored in persistent/session storage, URL href, or hash
    expect(localStorage.getItem('share123')).toBeNull();
    expect(sessionStorage.getItem('share123')).toBeNull();
    expect(window.location.href).not.toContain('up_o1_abcdef123456');
    expect(window.location.hash).not.toContain('up_o1_abcdef123456');
    expect(JSON.stringify(window.history.state || {})).not.toContain('up_o1_abcdef123456');
  });

  it('triggers onReset when clicking New share', () => {
    const onReset = vi.fn();
    renderWithProviders(
      <CreationSuccess
        shareId="share123"
        ownerToken="up_o1_abcdef123456"
        onReset={onReset}
      />,
    );

    const newShareBtn = screen.getByRole('button', { name: /new share/i });
    fireEvent.click(newShareBtn);

    expect(onReset).toHaveBeenCalled();
  });
});
