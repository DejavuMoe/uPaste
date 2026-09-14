import React, { useState } from 'react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeSanitize from 'rehype-sanitize';
import { CopyButton } from '../../components/CopyButton';
import {
  formatContentSize,
  formatExpiration,
  isSafeUrl,
} from './viewerHelpers';

export interface MarkdownViewerProps {
  shareId: string;
  content: string;
  isEncrypted: boolean;
  expiresAt: string | null;
  byteLength?: number;
}

export const MarkdownViewer: React.FC<MarkdownViewerProps> = ({
  shareId,
  content,
  isEncrypted,
  expiresAt,
  byteLength,
}) => {
  const [viewMode, setViewMode] = useState<'rendered' | 'source'>('rendered');

  const actualBytes =
    byteLength ?? new TextEncoder().encode(content).byteLength;

  const sizeLabel = formatContentSize(actualBytes);
  const expirationLabel = formatExpiration(expiresAt);

  const customComponents: Components = {
    // Override img to strictly prevent remote media requests and render safe text/link
    img: ({ node: _node, src, alt }) => {
      const safe = isSafeUrl(src);
      const label = alt ? `[Image: ${alt}]` : '[Image]';
      if (safe && src) {
        return (
          <a
            href={src}
            target="_blank"
            rel="noopener noreferrer nofollow"
            className="markdown-image-link"
          >
            {label}
          </a>
        );
      }
      return <span className="markdown-image-fallback">{label}</span>;
    },

    // Override a to enforce safe schemes and external link security
    a: ({ node: _node, href, children, ...props }) => {
      if (isSafeUrl(href)) {
        return (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer nofollow"
            {...props}
          >
            {children}
          </a>
        );
      }
      return <span className="neutralized-link">{children}</span>;
    },

    // Override input to enforce read-only disabled checkboxes for task lists
    input: ({ node: _node, type, checked, ...props }) => {
      if (type === 'checkbox') {
        return <input type="checkbox" checked={checked} disabled readOnly {...props} />;
      }
      return null;
    },

    // Wrap tables to enable horizontal scroll on overflow without breaking page bounds
    table: ({ node: _node, children, ...props }) => (
      <div className="table-responsive-wrapper">
        <table {...props}>{children}</table>
      </div>
    ),

    // Keyboard-accessible scrollable pre blocks
    pre: ({ node: _node, children, ...props }) => (
      <pre tabIndex={0} {...props}>
        {children}
      </pre>
    ),
  };

  return (
    <article className="viewer-card">
      <header className="viewer-meta-toolbar">
        <div className="viewer-meta">
          <span>Markdown</span>
          <span className="meta-separator" aria-hidden="true">·</span>
          <span>{sizeLabel}</span>
          <span className="meta-separator" aria-hidden="true">·</span>
          <span>{expirationLabel}</span>
        </div>

        <div className="viewer-actions">
          <div
            className="segmented-control"
            role="group"
            aria-label="Markdown view mode"
          >
            <button
              type="button"
              className={`segmented-button ${
                viewMode === 'rendered' ? 'active' : ''
              }`}
              onClick={() => setViewMode('rendered')}
              aria-pressed={viewMode === 'rendered'}
            >
              Rendered
            </button>
            <button
              type="button"
              className={`segmented-button ${
                viewMode === 'source' ? 'active' : ''
              }`}
              onClick={() => setViewMode('source')}
              aria-pressed={viewMode === 'source'}
            >
              Source
            </button>
          </div>

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
        {viewMode === 'source' ? (
          <pre
            className="viewer-content markdown-source-content"
            tabIndex={0}
            aria-label="Markdown source content"
          >
            <code>{content}</code>
          </pre>
        ) : (
          <div className="viewer-content markdown-body">
            <Markdown
              remarkPlugins={[remarkGfm]}
              rehypePlugins={[rehypeSanitize]}
              skipHtml
              components={customComponents}
            >
              {content}
            </Markdown>
          </div>
        )}
      </div>
    </article>
  );
};
