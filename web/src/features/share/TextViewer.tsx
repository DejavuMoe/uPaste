import React, { useState } from 'react';
import { CopyButton } from '../../components/CopyButton';
import {
  formatFormatLabel,
  formatContentSize,
  formatExpiration,
} from './viewerHelpers';

export interface TextViewerProps {
  shareId: string;
  format: 'PLAIN' | 'SOURCE';
  content: string;
  isEncrypted: boolean;
  expiresAt: string | null;
  byteLength?: number;
}

export const TextViewer: React.FC<TextViewerProps> = ({
  shareId,
  format,
  content,
  isEncrypted,
  expiresAt,
  byteLength,
}) => {
  const [isWrapped, setIsWrapped] = useState<boolean>(false);

  const actualBytes =
    byteLength ?? new TextEncoder().encode(content).byteLength;

  const formatLabel = formatFormatLabel(format);
  const sizeLabel = formatContentSize(actualBytes);
  const expirationLabel = formatExpiration(expiresAt);

  return (
    <article className="viewer-card">
      <header className="viewer-meta-toolbar">
        <div className="viewer-meta">
          <span>{formatLabel}</span>
          <span className="meta-separator" aria-hidden="true">·</span>
          <span>{sizeLabel}</span>
          <span className="meta-separator" aria-hidden="true">·</span>
          <span>{expirationLabel}</span>
        </div>
        <div className="viewer-actions">
          {format === 'SOURCE' && (
            <button
              type="button"
              className={`btn btn-secondary ${isWrapped ? 'active' : ''}`}
              onClick={() => setIsWrapped((prev) => !prev)}
              aria-pressed={isWrapped}
            >
              Wrap lines
            </button>
          )}
          <CopyButton textToCopy={content} />
          {!isEncrypted && (
            <a
              href={`/raw/${shareId}`}
              className="btn btn-secondary"
              target="_blank"
              rel="noopener noreferrer"
            >
              Raw
            </a>
          )}
        </div>
      </header>

      <div className="viewer-body">
        {format === 'PLAIN' ? (
          <div className="viewer-content plain-text-content">{content}</div>
        ) : (
          <pre
            className={`viewer-content source-code-content ${
              isWrapped ? 'wrap-lines' : 'no-wrap'
            }`}
            tabIndex={0}
            aria-label="Source code content"
          >
            <code>{content}</code>
          </pre>
        )}
      </div>
    </article>
  );
};
