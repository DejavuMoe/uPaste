import React, { useState, useId, useRef, useEffect } from 'react';
import type { TextFormat, PrivacyMode } from '../../app/types';
import { createStandardText, createEncryptedText, ApiError } from '../../app/api';
import { createNewEncryptedText, MAX_CONTENT_BYTES } from '../../crypto/encryptedText';
import { Button } from '../../components/Button';
import { ExpirationField, type ExpirationValue } from '../../components/ExpirationField';

export interface TextCreateSuccessData {
  shareId: string;
  ownerToken: string;
  encryptionKey?: string;
}

export interface TextCreateFormProps {
  onSuccess: (data: TextCreateSuccessData) => void;
  onDirtyChange?: (isDirty: boolean) => void;
}

const WARNING_BYTES_THRESHOLD = 943_718; // 90% of 1 MiB

function formatByteCount(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  return `${(bytes / 1024).toFixed(1)} KB`;
}

export const TextCreateForm: React.FC<TextCreateFormProps> = ({ onSuccess, onDirtyChange }) => {
  const [format, setFormat] = useState<TextFormat>('PLAIN');
  const [privacy, setPrivacy] = useState<PrivacyMode>('STANDARD');
  const [content, setContent] = useState<string>('');
  const [expiration, setExpiration] = useState<ExpirationValue>({
    preset: 'never',
    resolveExpiresAt: () => null,
  });
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const isMountedRef = useRef<boolean>(true);
  const editorId = useId();

  // Abort in-flight requests on unmount
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, []);

  // UTF-8 byte calculation
  const byteLength = new TextEncoder().encode(content).byteLength;
  const isOverLimit = byteLength > MAX_CONTENT_BYTES;
  const isNearLimit = byteLength >= WARNING_BYTES_THRESHOLD && !isOverLimit;
  const isEmpty = byteLength === 0;
  const hasExpirationError = !!expiration.error;
  const canSubmit = !isEmpty && !isOverLimit && !hasExpirationError && !isSubmitting;

  // Report dirty state whenever content byteLength changes
  useEffect(() => {
    onDirtyChange?.(byteLength > 0);
  }, [byteLength, onDirtyChange]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit || isSubmitting) return;

    setIsSubmitting(true);
    setErrorMessage(null);

    const expiresAt = expiration.resolveExpiresAt();
    const controller = new AbortController();
    abortControllerRef.current = controller;

    try {
      if (privacy === 'STANDARD') {
        const res = await createStandardText(
          {
            payload_kind: 'TEXT',
            privacy_mode: 'STANDARD',
            text: {
              format,
              content,
            },
            expires_at: expiresAt,
          },
          controller.signal,
        );

        if (!isMountedRef.current) return;
        onSuccess({
          shareId: res.share.id,
          ownerToken: res.owner_token,
        });
      } else {
        // Encrypted Text Flow:
        // Encrypt in browser; server receives only ciphertext + nonce
        const { payload, fragment } = await createNewEncryptedText(format, content);
        // Extract key from fragment: #up_e1_<key>
        const encryptionKey = fragment.replace(/^#up_e1_/, '');

        const res = await createEncryptedText(
          {
            payload_kind: 'TEXT',
            privacy_mode: 'ENCRYPTED',
            encrypted_text: {
              protocol: 'UPASTE_AES_GCM_V1',
              nonce: payload.nonce,
              ciphertext: payload.ciphertext,
            },
            expires_at: expiresAt,
          },
          controller.signal,
        );

        if (!isMountedRef.current) return;
        onSuccess({
          shareId: res.share.id,
          ownerToken: res.owner_token,
          encryptionKey,
        });
      }
    } catch (err: any) {
      if (err.name === 'AbortError') return;
      if (!isMountedRef.current) return;

      if (err instanceof ApiError) {
        if (err.status === 429 && err.retryAfterSeconds) {
          setErrorMessage(
            `Rate limit exceeded. Please wait ${err.retryAfterSeconds} seconds before trying again.`,
          );
        } else {
          setErrorMessage(err.message || `Error ${err.status}: ${err.code}`);
        }
      } else if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage('An unexpected error occurred. Please try again.');
      }
    } finally {
      if (isMountedRef.current) {
        setIsSubmitting(false);
      }
      abortControllerRef.current = null;
    }
  };

  return (
    <form className="create-form text-create-form" onSubmit={handleSubmit} noValidate>
      <div className="form-toolbar">
        <div className="toolbar-group">
          {/* Format Selector */}
          <div className="toolbar-item">
            <span className="control-label" id="format-label">Format</span>
            <div className="segmented-control" role="radiogroup" aria-labelledby="format-label">
              <button
                type="button"
                role="radio"
                aria-checked={format === 'PLAIN'}
                className={`segmented-button ${format === 'PLAIN' ? 'active' : ''}`}
                onClick={() => setFormat('PLAIN')}
                disabled={isSubmitting}
              >
                Plain text
              </button>
              <button
                type="button"
                role="radio"
                aria-checked={format === 'SOURCE'}
                className={`segmented-button ${format === 'SOURCE' ? 'active' : ''}`}
                onClick={() => setFormat('SOURCE')}
                disabled={isSubmitting}
              >
                Source
              </button>
              <button
                type="button"
                role="radio"
                aria-checked={format === 'MARKDOWN'}
                className={`segmented-button ${format === 'MARKDOWN' ? 'active' : ''}`}
                onClick={() => setFormat('MARKDOWN')}
                disabled={isSubmitting}
              >
                Markdown
              </button>
            </div>
          </div>

          {/* Privacy Selector */}
          <div className="toolbar-item">
            <span className="control-label" id="privacy-label">Privacy</span>
            <div className="segmented-control" role="radiogroup" aria-labelledby="privacy-label">
              <button
                type="button"
                role="radio"
                aria-checked={privacy === 'STANDARD'}
                className={`segmented-button ${privacy === 'STANDARD' ? 'active' : ''}`}
                onClick={() => setPrivacy('STANDARD')}
                disabled={isSubmitting}
              >
                Standard
              </button>
              <button
                type="button"
                role="radio"
                aria-checked={privacy === 'ENCRYPTED'}
                className={`segmented-button ${privacy === 'ENCRYPTED' ? 'active' : ''}`}
                onClick={() => setPrivacy('ENCRYPTED')}
                disabled={isSubmitting}
              >
                Encrypted
              </button>
            </div>
          </div>
        </div>

        {/* Expiration Selector */}
        <div className="toolbar-item toolbar-expiration">
          <span className="control-label" id="expiration-label">Expires</span>
          <ExpirationField
            value={expiration.preset}
            onChange={setExpiration}
            disabled={isSubmitting}
          />
        </div>
      </div>

      {privacy === 'ENCRYPTED' && (
        <div className="privacy-notice" role="note">
          Encrypted in this browser. Anyone with the complete link can read it. The server cannot
          recover a lost decryption key.
        </div>
      )}

      {errorMessage && (
        <div className="form-error-banner" role="alert">
          {errorMessage}
        </div>
      )}

      {/* Editor Area */}
      <div className="editor-container">
        <label htmlFor={editorId} className="sr-only">
          Share text content
        </label>
        <textarea
          id={editorId}
          className={`editor-textarea ${format === 'SOURCE' ? 'font-mono' : 'font-sans'}`}
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="Paste or type content here…"
          disabled={isSubmitting}
          rows={16}
          spellCheck={format !== 'SOURCE'}
          aria-invalid={isOverLimit}
          aria-describedby="byte-counter"
        />

        <div className="editor-footer">
          <div
            id="byte-counter"
            className={`byte-counter ${
              isOverLimit ? 'counter-danger' : isNearLimit ? 'counter-warning' : ''
            }`}
            aria-live="polite"
          >
            {formatByteCount(byteLength)} / 1 MiB
            {isOverLimit && (
              <span className="counter-error-text"> (Limit exceeded by {byteLength - MAX_CONTENT_BYTES} bytes)</span>
            )}
          </div>

          <Button
            type="submit"
            variant="primary"
            disabled={!canSubmit}
            loading={isSubmitting}
          >
            {isSubmitting ? 'Creating…' : 'Create share'}
          </Button>
        </div>
      </div>
    </form>
  );
};
