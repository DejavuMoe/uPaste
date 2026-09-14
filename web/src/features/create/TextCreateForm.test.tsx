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

  it('disables submit button when content is empty', () => {
    render(<TextCreateForm onSuccess={vi.fn()} />);

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    expect(submitBtn).toBeDisabled();

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'non-empty' } });

    expect(submitBtn).not.toBeDisabled();
  });

  it('enforces 1 MiB limit and shows warning at 90%', () => {
    render(<TextCreateForm onSuccess={vi.fn()} />);
    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    const submitBtn = screen.getByRole('button', { name: /create share/i });

    // 950,000 bytes (exceeds 90% threshold of 943,718)
    const largeContent = 'a'.repeat(950_000);
    fireEvent.change(textarea, { target: { value: largeContent } });

    const counter = screen.getByText(/927\.7 KB \/ 1 MiB/);
    expect(counter).toHaveClass('counter-warning');
    expect(submitBtn).not.toBeDisabled();

    // Exceed 1 MiB limit (1,048,577 bytes)
    const overLimitContent = 'a'.repeat(1_048_577);
    fireEvent.change(textarea, { target: { value: overLimitContent } });

    expect(counter).toHaveClass('counter-danger');
    expect(screen.getByText(/Limit exceeded by 1 bytes/)).toBeInTheDocument();
    expect(submitBtn).toBeDisabled();
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

  it('displays API error inline and preserves draft content', async () => {
    vi.spyOn(api, 'createStandardText').mockRejectedValue(
      new api.ApiError(400, 'invalid_request', 'Invalid share content'),
    );

    render(<TextCreateForm onSuccess={vi.fn()} />);

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'preserve this draft' } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Invalid share content');
    });

    // Verify draft text is still in the textarea
    expect((textarea as HTMLTextAreaElement).value).toBe('preserve this draft');
  });
});
