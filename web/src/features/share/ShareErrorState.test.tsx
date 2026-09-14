import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { ShareErrorState } from './ShareErrorState';

describe('ShareErrorState', () => {
  it('renders missing_key state with Link to new share', () => {
    render(
      <MemoryRouter>
        <ShareErrorState type="missing_key" />
      </MemoryRouter>
    );

    expect(screen.getByText(/Decryption key missing/)).toBeInTheDocument();
    expect(
      screen.getByText(/This encrypted share cannot be read without the complete link/i)
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'New share' })).toHaveAttribute('href', '/');
  });

  it('renders decrypt_error state with Link to new share', () => {
    render(
      <MemoryRouter>
        <ShareErrorState type="decrypt_error" />
      </MemoryRouter>
    );

    expect(screen.getByText(/Unable to decrypt this share/)).toBeInTheDocument();
    expect(
      screen.getByText(/The link may be incomplete, or the encrypted data may have been modified/i)
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'New share' })).toHaveAttribute('href', '/');
  });

  it('renders not_found state with Link to create new share', () => {
    render(
      <MemoryRouter>
        <ShareErrorState type="not_found" />
      </MemoryRouter>
    );

    expect(screen.getByText('Share not found')).toBeInTheDocument();
    expect(
      screen.getByText(/The link may be incorrect, or the share may have been deleted/i)
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Create new share' })).toHaveAttribute('href', '/');
  });

  it('renders expired state with Link to create new share', () => {
    render(
      <MemoryRouter>
        <ShareErrorState type="expired" />
      </MemoryRouter>
    );

    expect(screen.getByText('This share has expired')).toBeInTheDocument();
    expect(
      screen.getByText(/Expired shares are no longer accessible and cannot be recovered/i)
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Create new share' })).toHaveAttribute('href', '/');
  });

  it('renders rate_limited state with retry button', async () => {
    const user = userEvent.setup();
    const retryMock = vi.fn();
    render(
      <MemoryRouter>
        <ShareErrorState type="rate_limited" retryAfterSeconds={30} onRetry={retryMock} />
      </MemoryRouter>
    );

    expect(screen.getByText('Too many requests')).toBeInTheDocument();
    expect(screen.getByText(/Please wait approximately 30 seconds before trying again/)).toBeInTheDocument();

    const tryAgainBtn = screen.getByRole('button', { name: 'Try again' });
    await user.click(tryAgainBtn);
    expect(retryMock).toHaveBeenCalledTimes(1);
  });

  it('renders network_error and server_error states with retry button', async () => {
    const user = userEvent.setup();
    const retryMock = vi.fn();
    const { rerender } = render(
      <MemoryRouter>
        <ShareErrorState type="network_error" onRetry={retryMock} />
      </MemoryRouter>
    );

    expect(screen.getByText('Could not reach server')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Try again' }));
    expect(retryMock).toHaveBeenCalledTimes(1);

    rerender(
      <MemoryRouter>
        <ShareErrorState type="server_error" onRetry={retryMock} />
      </MemoryRouter>
    );
    expect(screen.getByText('Service error')).toBeInTheDocument();
  });
});
