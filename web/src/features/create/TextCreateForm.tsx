import React, { useState, useId, useRef, useEffect } from 'react';
import type { TextFormat, PrivacyMode } from '../../app/types';
import { createStandardText, createEncryptedText, ApiError } from '../../app/api';
import { createNewEncryptedText, MAX_CONTENT_BYTES } from '../../crypto/encryptedText';
import { Button } from '../../components/Button';
import { ExpirationField, type ExpirationValue } from '../../components/ExpirationField';
import { ChallengeGate } from '../challenge/ChallengeGate';
import { useDeploymentConfig } from '../../app/config';
import { useLanguage, type Language } from '../../app/locale';
import { createCopy } from './createCopy';

export interface TextCreateSuccessData {
  shareId: string;
  ownerToken: string;
  encryptionKey?: string;
  kind: 'text';
  format: TextFormat;
  privacy: PrivacyMode;
  expiration: ExpirationValue['preset'];
}

export interface TextCreateFormProps {
  onSuccess: (data: TextCreateSuccessData) => void;
  onDirtyChange?: (isDirty: boolean) => void;
  onByteCountChange?: (bytes: number) => void;
  language?: Language;
}

function formatByteCount(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  return `${(bytes / 1024).toFixed(1)} KiB`;
}

export const TextCreateForm: React.FC<TextCreateFormProps> = ({ onSuccess, onDirtyChange, onByteCountChange, language: languageProp }) => {
  const currentLanguage = useLanguage().language;
  const language = languageProp ?? currentLanguage;
  const t = createCopy[language];
  const [format, setFormat] = useState<TextFormat>('PLAIN');
  const [privacy, setPrivacy] = useState<PrivacyMode>('STANDARD');
  const [content, setContent] = useState<string>('');
  const [expiration, setExpiration] = useState<ExpirationValue>({
    preset: 'never',
    resolveExpiresAt: () => null,
  });
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const { config } = useDeploymentConfig();
  const challengeConfig = config.challenge;
  const requiresChallenge = !!challengeConfig;
  const [challengeToken, setChallengeToken] = useState<string | null>(null);
  const [challengeResetKey, setChallengeResetKey] = useState(0);
  const expirationPolicy = {
    mode: config.deployment_mode,
    defaultSeconds: config.retention?.default_seconds,
    maxSeconds: config.retention?.max_seconds,
  };

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
  const isEmpty = byteLength === 0;
  const hasExpirationError = !!expiration.error;
  const canSubmit = !isEmpty && !isOverLimit && !hasExpirationError && !isSubmitting && (!requiresChallenge || challengeToken !== null);

  // Report dirty state whenever content byteLength changes
  useEffect(() => {
    onDirtyChange?.(byteLength > 0);
    onByteCountChange?.(byteLength);
  }, [byteLength, onDirtyChange, onByteCountChange]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit || isSubmitting) return;
    if (requiresChallenge && !challengeToken) {
      setErrorMessage(t.challenge);
      return;
    }

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
          ...(requiresChallenge ? [challengeToken, controller.signal] : [controller.signal]),
        );

        if (!isMountedRef.current) return;
        onSuccess({
          shareId: res.share.id,
          ownerToken: res.owner_token,
          kind: 'text', format, privacy, expiration: expiration.preset,
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
          ...(requiresChallenge ? [challengeToken, controller.signal] : [controller.signal]),
        );

        if (!isMountedRef.current) return;
        onSuccess({
          shareId: res.share.id,
          ownerToken: res.owner_token,
          encryptionKey,
          kind: 'text', format, privacy, expiration: expiration.preset,
        });
      }
    } catch (err: any) {
      if (err.name === 'AbortError') return;
      if (!isMountedRef.current) return;

      if (err instanceof ApiError) {
        if (err.status === 429 && err.retryAfterSeconds) {
          setErrorMessage(t.rateLimit(err.retryAfterSeconds));
        } else {
          setErrorMessage(err.message || `Error ${err.status}: ${err.code}`);
        }
      } else if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage(t.unexpected);
      }
    } finally {
      if (isMountedRef.current) {
        setIsSubmitting(false);
        if (requiresChallenge) {
          setChallengeToken(null);
          setChallengeResetKey((key) => key + 1);
        }
      }
      abortControllerRef.current = null;
    }
  };

  return (
    <form className="create-form text-create-form" onSubmit={handleSubmit} noValidate>
      <div className="create-editor-area">
        <label htmlFor={editorId} className="sr-only">{t.content}</label>
        <textarea
          id={editorId}
          className={format === 'SOURCE' ? 'font-mono' : 'font-sans'}
          value={content}
          onChange={(event) => setContent(event.target.value)}
          onKeyDown={(event) => {
            if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
              event.preventDefault();
              event.currentTarget.form?.requestSubmit();
            }
          }}
          placeholder={t.placeholder}
          disabled={isSubmitting}
          rows={10}
          spellCheck={format !== 'SOURCE'}
          aria-invalid={isOverLimit}
          aria-describedby="byte-counter"
        />
      </div>
      <span id="byte-counter" className="sr-only">
        {formatByteCount(byteLength)} / 1 MiB
        {isOverLimit && ` ${t.limitExceeded(byteLength - MAX_CONTENT_BYTES)}`}
      </span>

      <div className="create-controls">
        <div className="create-setting">
          <label htmlFor="create-format">{t.format}</label>
          <select id="create-format" className="form-select" value={format} disabled={isSubmitting} onChange={(event) => setFormat(event.target.value as TextFormat)}>
            <option value="PLAIN">{t.plain}</option>
            <option value="SOURCE">{t.source}</option>
            <option value="MARKDOWN">{t.markdown}</option>
          </select>
        </div>
        <fieldset className="create-setting create-privacy">
          <legend>{t.privacy}</legend>
          <label><input type="radio" name="privacy" value="STANDARD" checked={privacy === 'STANDARD'} disabled={isSubmitting} onChange={() => setPrivacy('STANDARD')} />{t.standard}</label>
          <label><input type="radio" name="privacy" value="ENCRYPTED" checked={privacy === 'ENCRYPTED'} disabled={isSubmitting} onChange={() => setPrivacy('ENCRYPTED')} />{t.encrypted}</label>
        </fieldset>
        <div className="create-setting">
          <label htmlFor="create-expiration">{t.expires}</label>
          <ExpirationField id="create-expiration" className="create-expiration" value={expiration.preset} onChange={setExpiration} disabled={isSubmitting} policy={expirationPolicy} language={language} />
        </div>
      </div>

      {privacy === 'ENCRYPTED' && <p className="create-note" role="note">{t.encryptedNote}</p>}
      {errorMessage && <p className="create-error" role="alert">{errorMessage}</p>}
      {isOverLimit && <p className="create-error" role="alert">{t.limitExceeded(byteLength - MAX_CONTENT_BYTES)}</p>}
      {requiresChallenge && challengeConfig && <ChallengeGate challenge={challengeConfig} onToken={setChallengeToken} resetKey={challengeResetKey} language={language} />}
      <div className="create-submit">
        <Button type="submit" variant="primary" disabled={!canSubmit} loading={isSubmitting}>{isSubmitting ? t.creating : t.create}</Button>
      </div>
    </form>
  );
};
