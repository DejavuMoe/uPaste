import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import React from 'react';
import { TextCreateForm } from './TextCreateForm';
import * as api from '../../app/api';

describe('TextCreateForm', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('measures bytes using UTF-8 TextEncoder, not string length', () => {
    render(<TextCreateForm onSuccess={vi.fn()} />);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    // "Hello 🌍" is 7 chars in JS (or 8 with surrogate), but UTF-8 is:
    // 'Hello ' (6 bytes) + '🌍' (4 bytes) = 10 bytes
    fireEvent.change(textarea, { target: { value: 'Hello 🌍' } });

    const counter = screen.getByText(/10 B \/ 1 MiB/);
    expect(counter).toBeInTheDocument();
  });

  it('correctly counts multi-byte CJK characters as UTF-8 bytes', () => {
    render(<TextCreateForm onSuccess={vi.fn()} />);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    // "你好世界" is 4 characters, 3 bytes each = 12 bytes
    fireEvent.change(textarea, { target: { value: '你好世界' } });

    expect(screen.getByText(/12 B \/ 1 MiB/)).toBeInTheDocument();
  });

  it('disables submit button when content is empty', () => {
    render(<TextCreateForm onSuccess={vi.fn()} />);

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    expect(submitBtn).toBeDisabled();

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'non-empty' } });

    expect(submitBtn).not.toBeDisabled();
  });

  it('permits whitespace-only content as valid non-empty input', async () => {
    const createSpy = vi.spyOn(api, 'createStandardText').mockResolvedValue({
      share: {
        id: 'ws-123',
        payload_kind: 'TEXT',
        privacy_mode: 'STANDARD',
        created_at: '2026-09-14T00:00:00Z',
        updated_at: '2026-09-14T00:00:00Z',
        expires_at: null,
      },
      owner_token: 'up_o1_tok',
    });

    render(<TextCreateForm onSuccess={vi.fn()} />);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: '   \n\t  ' } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    expect(submitBtn).not.toBeDisabled();

    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(createSpy).toHaveBeenCalled();
    });
  });

  it('accepts exactly 1,048,576 UTF-8 bytes and rejects 1,048,577 bytes', () => {
    render(<TextCreateForm onSuccess={vi.fn()} />);
    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    const submitBtn = screen.getByRole('button', { name: /create share/i });

    // Exactly 1 MiB (1,048,576 bytes)
    const exactMaxContent = 'a'.repeat(1_048_576);
    fireEvent.change(textarea, { target: { value: exactMaxContent } });

    expect(submitBtn).not.toBeDisabled();
    expect(screen.queryByText(/limit exceeded/i)).not.toBeInTheDocument();

    // 1 MiB + 1 byte (1,048,577 bytes)
    const overLimitContent = 'a'.repeat(1_048_577);
    fireEvent.change(textarea, { target: { value: overLimitContent } });

    expect(submitBtn).toBeDisabled();
    expect(screen.getByText(/Limit exceeded by 1 bytes/)).toBeInTheDocument();
  });

  it('renders visible labels for Format, Privacy, and Expires', () => {
    render(<TextCreateForm onSuccess={vi.fn()} />);

    expect(screen.getByText('Format')).toBeInTheDocument();
    expect(screen.getByText('Privacy')).toBeInTheDocument();
    expect(screen.getByText('Expires')).toBeInTheDocument();
  });

  it('submits Standard Text and invokes onSuccess callback', async () => {
    const onSuccess = vi.fn();
    const createSpy = vi.spyOn(api, 'createStandardText').mockResolvedValue({
      share: {
        id: 'share-std-123',
        payload_kind: 'TEXT',
        privacy_mode: 'STANDARD',
        created_at: '2026-09-14T00:00:00Z',
        updated_at: '2026-09-14T00:00:00Z',
        expires_at: null,
      },
      owner_token: 'up_o1_tok123',
    });

    render(<TextCreateForm onSuccess={onSuccess} />);

    // Switch format to Source
    const sourceBtn = screen.getByRole('radio', { name: /source/i });
    fireEvent.click(sourceBtn);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'const x = 42;' } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(createSpy).toHaveBeenCalledWith(
        {
          payload_kind: 'TEXT',
          privacy_mode: 'STANDARD',
          text: {
            format: 'SOURCE',
            content: 'const x = 42;',
          },
          expires_at: null,
        },
        expect.any(AbortSignal),
      );
    });

    expect(onSuccess).toHaveBeenCalledWith({
      shareId: 'share-std-123',
      ownerToken: 'up_o1_tok123',
    });
  });

  it('submits Encrypted Text without sending plaintext or key to server', async () => {
    const onSuccess = vi.fn();
    const createEncryptedSpy = vi.spyOn(api, 'createEncryptedText').mockResolvedValue({
      share: {
        id: 'share-enc-456',
        payload_kind: 'TEXT',
        privacy_mode: 'ENCRYPTED',
        created_at: '2026-09-14T00:00:00Z',
        updated_at: '2026-09-14T00:00:00Z',
        expires_at: null,
      },
      owner_token: 'up_o1_enctok456',
    });

    render(<TextCreateForm onSuccess={onSuccess} />);

    // Switch to Encrypted
    const encryptedRadio = screen.getByRole('radio', { name: /encrypted/i });
    fireEvent.click(encryptedRadio);

    // Verify privacy notice is displayed
    expect(screen.getByRole('note')).toHaveTextContent(/encrypted in this browser/i);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'super secret text' } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(createEncryptedSpy).toHaveBeenCalled();
    });

    const callArg = createEncryptedSpy.mock.calls[0][0];
    expect(callArg.payload_kind).toBe('TEXT');
    expect(callArg.privacy_mode).toBe('ENCRYPTED');
    expect(callArg.encrypted_text.protocol).toBe('UPASTE_AES_GCM_V1');
    expect(callArg.encrypted_text.nonce).toBeDefined();
    expect(callArg.encrypted_text.ciphertext).toBeDefined();

    // CRITICAL: Verify NO plaintext or key is sent in payload
    expect((callArg as any).text).toBeUndefined();
    expect((callArg as any).content).toBeUndefined();
    expect((callArg as any).key).toBeUndefined();
    expect((callArg as any).encrypted_text.key).toBeUndefined();

    // Verify onSuccess received the encryption key
    expect(onSuccess).toHaveBeenCalledWith(
      expect.objectContaining({
        shareId: 'share-enc-456',
        ownerToken: 'up_o1_enctok456',
        encryptionKey: expect.any(String),
      }),
    );
  });

  it('preserves draft content and re-enables button after 429 rate limit error', async () => {
    vi.spyOn(api, 'createStandardText').mockRejectedValue(
      new api.ApiError(429, 'rate_limit_exceeded', 'Rate limit exceeded', 30),
    );

    render(<TextCreateForm onSuccess={vi.fn()} />);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'my important draft' } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(
        'Rate limit exceeded. Please wait 30 seconds before trying again.',
      );
    });

    // Draft text preserved
    expect((textarea as HTMLTextAreaElement).value).toBe('my important draft');
    // Button re-enabled after request settles
    expect(submitBtn).not.toBeDisabled();
  });

  it('prevents duplicate submissions while request is in-flight', async () => {
    let resolveRequest: any;
    const pendingPromise = new Promise((resolve) => {
      resolveRequest = resolve;
    });

    const createSpy = vi.spyOn(api, 'createStandardText').mockImplementation(() => pendingPromise as any);

    render(<TextCreateForm onSuccess={vi.fn()} />);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'single submission test' } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });

    // First click initiates request
    fireEvent.click(submitBtn);
    expect(createSpy).toHaveBeenCalledTimes(1);

    // Second click while in-flight should be ignored
    fireEvent.click(submitBtn);
    expect(createSpy).toHaveBeenCalledTimes(1);

    // Resolve request
    resolveRequest({
      share: {
        id: 'dup-123',
        payload_kind: 'TEXT',
        privacy_mode: 'STANDARD',
        created_at: '2026-09-14T00:00:00Z',
        updated_at: '2026-09-14T00:00:00Z',
        expires_at: null,
      },
      owner_token: 'tok',
    });
  });

  it('aborts in-flight request when component unmounts', () => {
    let capturedSignal: AbortSignal | undefined;
    (vi.spyOn(api, 'createStandardText') as any).mockImplementation(async (_req: any, signal: any) => {
      capturedSignal = signal;
      return new Promise(() => {});
    });

    const { unmount } = render(<TextCreateForm onSuccess={vi.fn()} />);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'unmount test' } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    expect(capturedSignal).toBeDefined();
    expect(capturedSignal?.aborted).toBe(false);

    unmount();

    expect(capturedSignal?.aborted).toBe(true);
  });
});
