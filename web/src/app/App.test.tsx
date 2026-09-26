import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { appRoutes } from './App';
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
          <Helper />
          <RouterProvider router={createMemoryRouter(appRoutes, { initialEntries: [initialPath] })} />
        </OwnerCapabilityProvider>
      </ThemeProvider>,
    );
  };

  it('renders CreatePage on root / route in Chinese by default', () => {
    renderRoute('/');

    expect(document.title).toBe('新建分享 · uPaste');
    expect(document.documentElement.lang).toBe('zh-CN');
    expect(screen.getByRole('heading', { level: 1, name: '新建分享' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '文本' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '文件' })).toBeInTheDocument();

    // Text form is active by default
    expect(screen.getByRole('textbox', { name: '内容' })).toBeInTheDocument();
  });

  it('renders ShareRoute on /s/:id route with title "Share · uPaste"', () => {
    renderRoute('/s/test-share-id-123');

    expect(document.title).toBe('Share · uPaste');
    expect(document.documentElement.lang).toBe('en');
    expect(screen.getByText('Loading share…')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'New share' })).toBeInTheDocument();

    // Internal Phase labels must NOT be exposed
    expect(document.body.textContent).not.toMatch(/Phase 7[ABC]/);
  });

  it('renders ManageRoute shell on /manage/:id route with title "Manage share · uPaste"', () => {
    renderRoute('/manage/test-manage-id-456');

    expect(document.title).toBe('Manage share · uPaste');
    expect(screen.getByText('Loading share…')).toBeInTheDocument();

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

  it('does not display in-memory capability characters while management loads', () => {
    renderRoute('/manage/share-with-cap', (caps) => {
      caps.remember('share-with-cap', 'up_o1_secret_capability_token');
    });

    expect(document.body.textContent).not.toContain('up_o1_secret_capability_token');
    expect(document.body.textContent).not.toContain('••••');
  });
});
