import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useLocation, Link } from 'react-router';
import { getShare, ApiError } from '../../app/api';
import type { ShareMetadata, TextFormat } from '../../app/types';
import { decryptWithFragment, type EncryptedText } from '../../crypto/encryptedText';
import { useDocumentTitle } from '../../app/useDocumentTitle';
import { TextViewer } from './TextViewer';
import { MarkdownViewer } from './MarkdownViewer';
import { FileViewer } from './FileViewer';
import { ShareErrorState, type ShareErrorType } from './ShareErrorState';
import { sanitizeFilename } from './viewerHelpers';

type ViewerStatus = 'loading' | 'decrypting' | 'ready' | 'error';

export const ShareRoute: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();

  const [status, setStatus] = useState<ViewerStatus>('loading');
  const [share, setShare] = useState<ShareMetadata | null>(null);
  const [decryptedText, setDecryptedText] = useState<EncryptedText | null>(null);
  const [errorType, setErrorType] = useState<ShareErrorType>('server_error');
  const [retryAfterSeconds, setRetryAfterSeconds] = useState<number | undefined>(undefined);
  const [retryCount, setRetryCount] = useState<number>(0);

  // Compute document title
  let documentTitle = 'Share · uPaste';
  if (status === 'error') {
    documentTitle = 'uPaste';
  } else if (status === 'ready' && share?.payload_kind === 'FILE' && share.file?.filename) {
    documentTitle = `${sanitizeFilename(share.file.filename)} · uPaste`;
  } else {
    documentTitle = 'Share · uPaste';
  }
  useDocumentTitle(documentTitle);

  const handleRetry = useCallback(() => {
    setRetryCount((prev) => prev + 1);
  }, []);

  useEffect(() => {
    if (!id) {
      setStatus('error');
      setErrorType('not_found');
      return;
    }

    let active = true;
    const controller = new AbortController();

    // Reset state on id change
    setStatus('loading');
    setShare(null);
    setDecryptedText(null);
    setRetryAfterSeconds(undefined);

    async function loadShare() {
      try {
        const response = await getShare(id!, controller.signal);
        if (!active) return;

        const fetchedShare = response.share;
        setShare(fetchedShare);

        if (fetchedShare.privacy_mode === 'ENCRYPTED') {
          setStatus('decrypting');
          const fragment = window.location.hash || location.hash;

          if (!fragment || fragment === '#' || !fragment.startsWith('#up_e1_') || !fetchedShare.encrypted_text) {
            setStatus('error');
            setErrorType('missing_key');
            return;
          }

          try {
            const decrypted = await decryptWithFragment(fragment, fetchedShare.encrypted_text as any);
            if (!active) return;
            setDecryptedText(decrypted);
            setStatus('ready');
          } catch {
            if (!active) return;
            setStatus('error');
            setErrorType('decrypt_error');
          }
        } else {
          setStatus('ready');
        }
      } catch (err: any) {
        if (!active || err.name === 'AbortError') return;

        setStatus('error');
        if (err instanceof ApiError) {
          if (err.status === 404) {
            setErrorType('not_found');
          } else if (err.status === 410) {
            setErrorType('expired');
          } else if (err.status === 429) {
            setErrorType('rate_limited');
            setRetryAfterSeconds(err.retryAfterSeconds);
          } else if (err.status >= 500) {
            setErrorType('server_error');
          } else {
            setErrorType('server_error');
          }
        } else {
          setErrorType('network_error');
        }
      }
    }

    loadShare();

    return () => {
      active = false;
      controller.abort();
    };
  }, [id, location.hash, retryCount]);

  return (
    <main className="page-container" id="main-content">
      <div className="viewer-top-nav">
        <div className="viewer-top-nav-start" />
        <div className="viewer-top-nav-end">
          <Link to="/" className="btn btn-secondary">
            New share
          </Link>
        </div>
      </div>

      {status === 'loading' && (
        <div className="viewer-card viewer-status-card" role="status">
          <p className="viewer-status-text">Loading share…</p>
        </div>
      )}

      {status === 'decrypting' && (
        <div className="viewer-card viewer-status-card" role="status">
          <p className="viewer-status-text">Decrypting…</p>
        </div>
      )}

      {status === 'error' && (
        <ShareErrorState
          type={errorType}
          retryAfterSeconds={retryAfterSeconds}
          onRetry={handleRetry}
        />
      )}

      {status === 'ready' && share && (
        <>
          {share.payload_kind === 'FILE' && <FileViewer share={share} />}

          {share.payload_kind === 'TEXT' && (
            <>
              {(() => {
                const isEncrypted = share.privacy_mode === 'ENCRYPTED';
                const format: TextFormat = isEncrypted
                  ? (decryptedText?.format ?? 'PLAIN')
                  : (share.text?.format ?? 'PLAIN');
                const content = isEncrypted
                  ? (decryptedText?.content ?? '')
                  : (share.text?.content ?? '');

                if (format === 'MARKDOWN') {
                  return (
                    <MarkdownViewer
                      shareId={share.id}
                      content={content}
                      isEncrypted={isEncrypted}
                      expiresAt={share.expires_at}
                    />
                  );
                }

                return (
                  <TextViewer
                    shareId={share.id}
                    format={format as 'PLAIN' | 'SOURCE'}
                    content={content}
                    isEncrypted={isEncrypted}
                    expiresAt={share.expires_at}
                  />
                );
              })()}
            </>
          )}
        </>
      )}
    </main>
  );
};
