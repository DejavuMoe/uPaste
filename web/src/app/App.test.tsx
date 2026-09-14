import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router';
import { AppRoutes } from './App';
import { ThemeProvider } from './theme';
import { OwnerCapabilityProvider } from './ownerCapabilities';

describe('App routing & layout', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  const renderRoute = (initialPath: string) => {
    return render(
      <ThemeProvider>
        <OwnerCapabilityProvider>
          <MemoryRouter initialEntries={[initialPath]}>
            <AppRoutes />
          </MemoryRouter>
        </OwnerCapabilityProvider>
      </ThemeProvider>,
    );
  };

  it('renders CreatePage on root / route with tabs', () => {
    renderRoute('/');

    expect(screen.getByRole('heading', { level: 1, name: /new share/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /text/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /file/i })).toBeInTheDocument();

    // Text form is active by default
    expect(screen.getByPlaceholderText(/paste or type content here/i)).toBeInTheDocument();

    // Switch to File tab
    fireEvent.click(screen.getByRole('tab', { name: /file/i }));
    expect(screen.getByText(/drop one file here/i)).toBeInTheDocument();
  });

  it('renders ShareRoute shell on /s/:id route', () => {
    renderRoute('/s/test-share-id-123');

    expect(screen.getByRole('heading', { level: 1, name: /share viewer/i })).toBeInTheDocument();
    expect(screen.getByText('test-share-id-123')).toBeInTheDocument();
  });

  it('renders ManageRoute shell on /manage/:id route', () => {
    renderRoute('/manage/test-manage-id-456');

    expect(screen.getByRole('heading', { level: 1, name: /manage share/i })).toBeInTheDocument();
    expect(screen.getByText('test-manage-id-456')).toBeInTheDocument();
  });
});
