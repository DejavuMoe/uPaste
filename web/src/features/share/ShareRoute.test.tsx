import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router';
import { ShareRoute } from './ShareRoute';
import * as api from '../../app/api';
import * as cryptoHelper from '../../crypto/encryptedText';
import type { ShareMetadata } from '../../app/types';

describe('ShareRoute', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.location.hash = '';
  });

  function renderShareRoute(initialPath: string) {
    return render(
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/s/:id" element={<ShareRoute />} />
          <Route path="/" element={<div>Home Page</div>} />
        </Routes>
      </MemoryRouter>
    );
  }

  it('shows loading state initially and renders standard plain text when loaded', async () => {
    const mockShare: ShareMetadata = {
      id: 'text-123',
      payload_kind: 'TEXT',
      privacy_mode: 'STANDARD',
      created_at: '2026-09-14T12:00:00Z',
      updated_at: '2026-09-14T12:00:00Z',
      expires_at: null,
      text: {
        format: 'PLAIN',
        content: 'Hello Plain World',
      },
    };

    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });

    renderShareRoute('/s/text-123');

    // Shows loading first
    expect(screen.getByText('Loading share…')).toBeInTheDocument();

    // Resolves to ready state
    await waitFor(() => {
      expect(screen.getByText('Hello Plain World')).toBeInTheDocument();
    });

    expect(screen.getByText('Plain text')).toBeInTheDocument();
    // document.title is applied by a passive effect, so it must be awaited
    // rather than read immediately after the DOM assertion above.
    await waitFor(() => expect(document.title).toBe('Share · uPaste'));
  });

  it('renders standard Markdown when text format is MARKDOWN', async () => {
    const mockShare: ShareMetadata = {
      id: 'md-123',
      payload_kind: 'TEXT',
      privacy_mode: 'STANDARD',
      created_at: '2026-09-14T12:00:00Z',
      updated_at: '2026-09-14T12:00:00Z',
      expires_at: null,
      text: {
        format: 'MARKDOWN',
        content: '# Markdown Document Title',
      },
    };

    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });

    renderShareRoute('/s/md-123');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Markdown Document Title' })).toBeInTheDocument();
    });

    await waitFor(() => expect(document.title).toBe('Share · uPaste'));
  });

  it('renders File viewer with filename in document title', async () => {
    const mockShare: ShareMetadata = {
      id: 'file-123',
      payload_kind: 'FILE',
      privacy_mode: 'STANDARD',
      created_at: '2026-09-14T12:00:00Z',
      updated_at: '2026-09-14T12:00:00Z',
      expires_at: null,
      file: {
        filename: 'invoice-2026.pdf',
        size: 2048,
        media_type: 'application/pdf',
        sha256: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
        download_url: 'http://127.0.0.1:8081/f/file-123',
      },
    };

    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });

    renderShareRoute('/s/file-123');

    await waitFor(() => {
      expect(screen.getByText('invoice-2026.pdf')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'Download file' })).toHaveAttribute(
      'href',
      'http://127.0.0.1:8081/f/file-123'
    );
    await waitFor(() => expect(document.title).toBe('invoice-2026.pdf · uPaste'));
  });

  it('decrypts encrypted text using fragment and never passes fragment to api', async () => {
    const mockShare: ShareMetadata = {
      id: 'enc-123',
      payload_kind: 'TEXT',
      privacy_mode: 'ENCRYPTED',
      created_at: '2026-09-14T12:00:00Z',
      updated_at: '2026-09-14T12:00:00Z',
      expires_at: null,
      encrypted_text: {
        protocol: 'UPASTE_AES_GCM_V1',
        nonce: '123456789012',
        ciphertext: 'abcdefghij',
      },
    };

    const getShareSpy = vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });
    const decryptSpy = vi.spyOn(cryptoHelper, 'decryptWithFragment').mockResolvedValue({
      format: 'PLAIN',
      content: 'Top Secret Cleartext',
    });

    const keyFragment = '#up_e1_YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY';
    window.location.hash = keyFragment;

    renderShareRoute(`/s/enc-123${keyFragment}`);

    await waitFor(() => {
      expect(screen.getByText('Top Secret Cleartext')).toBeInTheDocument();
    });

    // API should have been called with ID ONLY, without fragment
    expect(getShareSpy).toHaveBeenCalledWith('enc-123', expect.any(AbortSignal));
    // Decrypt should have been called with fragment
    expect(decryptSpy).toHaveBeenCalledWith(keyFragment, mockShare.encrypted_text);
    // Raw button must NOT exist
    expect(screen.queryByRole('link', { name: 'Raw' })).toBeNull();
    await waitFor(() => expect(document.title).toBe('Share · uPaste'));
  });

  it('displays "Decryption key missing" if URL fragment is absent', async () => {
    const mockShare: ShareMetadata = {
      id: 'enc-missing-key',
      payload_kind: 'TEXT',
      privacy_mode: 'ENCRYPTED',
      created_at: '2026-09-14T12:00:00Z',
      updated_at: '2026-09-14T12:00:00Z',
      expires_at: null,
      encrypted_text: {
        protocol: 'UPASTE_AES_GCM_V1',
        nonce: '123456789012',
        ciphertext: 'abcdefghij',
      },
    };

    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });
    const decryptSpy = vi.spyOn(cryptoHelper, 'decryptWithFragment');
    window.location.hash = '';

    renderShareRoute('/s/enc-missing-key');

    await waitFor(() => {
      expect(screen.getByText(/Decryption key missing/)).toBeInTheDocument();
    });

    expect(screen.getByText(/This encrypted share cannot be read without the complete link/)).toBeInTheDocument();
    expect(decryptSpy).not.toHaveBeenCalled();
    await waitFor(() => expect(document.title).toBe('uPaste'));
  });

  it('treats an empty hash as a missing key', async () => {
    const mockShare: ShareMetadata = {
      id: 'enc-empty-hash', payload_kind: 'TEXT', privacy_mode: 'ENCRYPTED',
      created_at: '2026-09-14T12:00:00Z', updated_at: '2026-09-14T12:00:00Z', expires_at: null,
      encrypted_text: { protocol: 'UPASTE_AES_GCM_V1', nonce: '123456789012', ciphertext: 'abcdefghij' },
    };
    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });
    const decryptSpy = vi.spyOn(cryptoHelper, 'decryptWithFragment');

    renderShareRoute('/s/enc-empty-hash#');

    await waitFor(() => expect(screen.getByText(/Decryption key missing/)).toBeInTheDocument());
    expect(decryptSpy).not.toHaveBeenCalled();
  });

  it.each(['#wrong', '#up_e1_invalid'])('treats malformed non-empty fragment %s as a decrypt error', async (fragment) => {
    const mockShare: ShareMetadata = {
      id: 'enc-malformed', payload_kind: 'TEXT', privacy_mode: 'ENCRYPTED',
      created_at: '2026-09-14T12:00:00Z', updated_at: '2026-09-14T12:00:00Z', expires_at: null,
      encrypted_text: { protocol: 'UPASTE_AES_GCM_V1', nonce: '123456789012', ciphertext: 'abcdefghij' },
    };
    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });
    vi.spyOn(cryptoHelper, 'decryptWithFragment').mockRejectedValue(new Error('invalid capability'));
    window.location.hash = fragment;

    renderShareRoute(`/s/enc-malformed${fragment}`);

    await waitFor(() => expect(screen.getByText(/Unable to decrypt this share/)).toBeInTheDocument());
    expect(screen.queryByText(/Decryption key missing/)).toBeNull();
  });

  it('displays "Unable to decrypt this share" if a wrong valid-shape key fails', async () => {
    const mockShare: ShareMetadata = {
      id: 'enc-corrupted',
      payload_kind: 'TEXT',
      privacy_mode: 'ENCRYPTED',
      created_at: '2026-09-14T12:00:00Z',
      updated_at: '2026-09-14T12:00:00Z',
      expires_at: null,
      encrypted_text: {
        protocol: 'UPASTE_AES_GCM_V1',
        nonce: '123456789012',
        ciphertext: 'abcdefghij',
      },
    };

    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });
    vi.spyOn(cryptoHelper, 'decryptWithFragment').mockRejectedValue(
      new Error('The operation failed for an operation-specific reason')
    );

    const keyFragment = '#up_e1_YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY';
    window.location.hash = keyFragment;

    renderShareRoute(`/s/enc-corrupted${keyFragment}`);

    await waitFor(() => {
      expect(screen.getByText(/Unable to decrypt this share/)).toBeInTheDocument();
    });

    // Error message must not display cryptographic details or stack traces
    expect(screen.queryByText(/operation-specific reason/i)).toBeNull();
    await waitFor(() => expect(document.title).toBe('uPaste'));
  });

  it('displays "Unable to decrypt this share" if ciphertext is tampered', async () => {
    const mockShare: ShareMetadata = {
      id: 'enc-tampered', payload_kind: 'TEXT', privacy_mode: 'ENCRYPTED',
      created_at: '2026-09-14T12:00:00Z', updated_at: '2026-09-14T12:00:00Z', expires_at: null,
      encrypted_text: { protocol: 'UPASTE_AES_GCM_V1', nonce: '123456789012', ciphertext: 'tampered' },
    };
    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });
    vi.spyOn(cryptoHelper, 'decryptWithFragment').mockRejectedValue(new Error('authentication failed'));
    window.location.hash = '#up_e1_YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY';

    renderShareRoute('/s/enc-tampered');

    await waitFor(() => expect(screen.getByText(/Unable to decrypt this share/)).toBeInTheDocument());
  });

  it.each([
    ['standard text without text', { payload_kind: 'TEXT', privacy_mode: 'STANDARD' }],
    ['encrypted text without encrypted_text', { payload_kind: 'TEXT', privacy_mode: 'ENCRYPTED' }],
    ['file without file', { payload_kind: 'FILE', privacy_mode: 'STANDARD' }],
  ] as const)('fails closed for malformed API payload: %s', async (_name, payload) => {
    const mockShare = {
      id: 'malformed', created_at: '2026-09-14T12:00:00Z', updated_at: '2026-09-14T12:00:00Z', expires_at: null, ...payload,
    } as ShareMetadata;
    vi.spyOn(api, 'getShare').mockResolvedValue({ share: mockShare });

    renderShareRoute('/s/malformed');

    await waitFor(() => expect(screen.getByText('Service error')).toBeInTheDocument());
    expect(screen.queryByText('Decryption key missing')).toBeNull();
  });

  it('handles 404 not found error', async () => {
    vi.spyOn(api, 'getShare').mockRejectedValue(
      new api.ApiError(404, 'not_found', 'share not found')
    );

    renderShareRoute('/s/missing-id');

    await waitFor(() => {
      expect(screen.getByText('Share not found')).toBeInTheDocument();
    });
    await waitFor(() => expect(document.title).toBe('uPaste'));
  });

  it('handles 410 expired error', async () => {
    vi.spyOn(api, 'getShare').mockRejectedValue(
      new api.ApiError(410, 'expired', 'share has expired')
    );

    renderShareRoute('/s/expired-id');

    await waitFor(() => {
      expect(screen.getByText('This share has expired')).toBeInTheDocument();
    });
    await waitFor(() => expect(document.title).toBe('uPaste'));
  });

  it('handles 429 rate limited error', async () => {
    vi.spyOn(api, 'getShare').mockRejectedValue(
      new api.ApiError(429, 'rate_limit_exceeded', 'rate limited', 60)
    );

    renderShareRoute('/s/limited-id');

    await waitFor(() => {
      expect(screen.getByText('Too many requests')).toBeInTheDocument();
      expect(screen.getByText(/60 seconds/)).toBeInTheDocument();
    });
    await waitFor(() => expect(document.title).toBe('uPaste'));
  });

  it('aborts previous fetch on unmount', () => {
    let abortSignal: AbortSignal | undefined;
    vi.spyOn(api, 'getShare').mockImplementation((_id, signal) => {
      abortSignal = signal;
      return new Promise(() => {}); // never resolves
    });

    const { unmount } = renderShareRoute('/s/hanging-id');
    expect(abortSignal?.aborted).toBe(false);

    unmount();
    expect(abortSignal?.aborted).toBe(true);
  });
});
