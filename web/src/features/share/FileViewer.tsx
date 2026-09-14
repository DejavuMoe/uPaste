import React from 'react';
import type { ShareMetadata } from '../../app/types';
import {
  formatFileSize,
  formatExpiration,
  sanitizeFilename,
} from './viewerHelpers';

export interface FileViewerProps {
  share: ShareMetadata;
}

export const FileViewer: React.FC<FileViewerProps> = ({ share }) => {
  const fileData = share.file;
  if (!fileData) {
    return null;
  }

  const cleanFilename = sanitizeFilename(fileData.filename);
  const sizeLabel = formatFileSize(fileData.size);
  const mediaType = fileData.media_type || 'application/octet-stream';

  let expirationDisplay = 'Never';
  if (share.expires_at) {
    const rawExp = formatExpiration(share.expires_at);
    expirationDisplay = rawExp.replace(/^Expires\s+/, '');
  }

  return (
    <article className="viewer-card file-viewer-card">
      <div className="file-viewer-header">
        <h2 className="file-viewer-filename" title={fileData.filename}>
          <span className="file-icon" aria-hidden="true">📄</span>
          <span className="filename-text">{cleanFilename}</span>
        </h2>
      </div>

      <dl className="file-meta-list">
        <div className="file-meta-row">
          <dt className="file-meta-key">Size:</dt>
          <dd className="file-meta-val">{sizeLabel}</dd>
        </div>
        <div className="file-meta-row">
          <dt className="file-meta-key">MIME:</dt>
          <dd className="file-meta-val font-mono">{mediaType}</dd>
        </div>
        <div className="file-meta-row">
          <dt className="file-meta-key">Expires:</dt>
          <dd className="file-meta-val">{expirationDisplay}</dd>
        </div>
      </dl>

      <div className="file-viewer-actions">
        <a
          href={fileData.download_url}
          download={cleanFilename}
          className="btn btn-primary btn-download"
        >
          Download file
        </a>
      </div>
    </article>
  );
};
