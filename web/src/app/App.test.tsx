import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router';
import { AppRoutes } from './App';
import { ThemeProvider } from './theme';
import { OwnerCapabilityProvider, useOwnerCapabilities } from './ownerCapabilities';

describe('App routing, titles, & security', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    document.title = '';
  });

  const renderRoute = (initialPath: string, onRender?: (caps: ReturnType<typeof useOwnerCapabilities>) => void) => {
    const Helper = () => {
      const caps = useOwnerCapabilities();
      onRender?.(caps);
      return null;
    };

    return render(
      <ThemeProvider>
        <OwnerCapabilityProvider>
          <MemoryRouter initialEntries={[initialPath]}>
            <Helper />
            <AppRoutes />
          </MemoryRouter>
        </OwnerCapabilityProvider>
      </ThemeProvider>,
    );
  };

  it('renders CreatePage on root / route with title "New share · uPaste"', () => {
    renderRoute('/');

    expect(document.title).toBe('New share · uPaste');
    expect(screen.getByRole('heading', { level: 1, name: /new share/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /text/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /file/i })).toBeInTheDocument();

    // Text form is active by default
    expect(screen.getByPlaceholderText(/paste or type content here/i)).toBeInTheDocument();
  });

  it('renders ShareRoute on /s/:id route with title "Share · uPaste"', () => {
    renderRoute('/s/test-share-id-123');

    expect(document.title).toBe('Share · uPaste');
    expect(screen.getByText('Loading share…')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'New share' })).toBeInTheDocument();

    // Internal Phase labels must NOT be exposed
    expect(document.body.textContent).not.toMatch(/Phase 7[ABC]/);
  });

  it('renders ManageRoute shell on /manage/:id route with title "Manage share · uPaste"', () => {
    renderRoute('/manage/test-manage-id-456');

    expect(document.title).toBe('Manage share · uPaste');
    expect(screen.getByRole('heading', { level: 1, name: /manage share/i })).toBeInTheDocument();
    expect(screen.getByText('test-manage-id-456')).toBeInTheDocument();
    expect(screen.getByText('Management editing is not available in this build.')).toBeInTheDocument();

    // Internal Phase labels must NOT be exposed
    expect(document.body.textContent).not.toMatch(/Phase 7[ABC]/);
  });

  it('does NOT render encrypted key fragment in ShareRoute DOM or title', () => {
    const secretFragment = '#up_e1_FAKE_SECRET_KEY_NEVER_LEAK_123';
    renderRoute(`/s/share-secret${secretFragment}`);

    // Fragment MUST NOT leak into visible DOM or body text
    expect(document.body.textContent).not.toContain('up_e1_FAKE_SECRET_KEY_NEVER_LEAK_123');
    expect(document.body.textContent).not.toContain('FAKE_SECRET_KEY_NEVER_LEAK_123');

    // Title MUST NOT contain the fragment or secret
    expect(document.title).toBe('Share · uPaste');
    expect(document.title).not.toContain('up_e1');
  });

  it('does NOT render encrypted key fragment in ManageRoute DOM or title', () => {
    const secretFragment = '#up_e1_FAKE_MANAGE_KEY_NEVER_LEAK_456';
    renderRoute(`/manage/share-manage${secretFragment}`);

    // Fragment MUST NOT leak into visible DOM or body text
    expect(document.body.textContent).not.toContain('up_e1_FAKE_MANAGE_KEY_NEVER_LEAK_456');
    expect(document.body.textContent).not.toContain('FAKE_MANAGE_KEY_NEVER_LEAK_456');

    // Title MUST NOT contain the fragment or secret
    expect(document.title).toBe('Manage share · uPaste');
    expect(document.title).not.toContain('up_e1');
  });

  it('displays neutral capability status in ManageRoute and does NOT display token characters', () => {
    renderRoute('/manage/share-with-cap', (caps) => {
      caps.remember('share-with-cap', 'up_o1_secret_capability_token');
    });

    expect(screen.getByText('Management token available for this session.')).toBeInTheDocument();

    // MUST NOT display any portion of the token (e.g. up_o1_•••• or raw)
    expect(document.body.textContent).not.toContain('up_o1_secret_capability_token');
    expect(document.body.textContent).not.toContain('••••');
  });
});
