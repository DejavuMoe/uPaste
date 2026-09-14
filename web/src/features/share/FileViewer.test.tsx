import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { FileViewer } from './FileViewer';
import type { ShareMetadata } from '../../app/types';

describe('FileViewer', () => {
  const sampleShare: ShareMetadata = {
    id: 'file-share-123',
    payload_kind: 'FILE',
    privacy_mode: 'STANDARD',
    created_at: '2026-09-14T12:00:00Z',
    updated_at: '2026-09-14T12:00:00Z',
    expires_at: '2026-09-20T12:00:00Z',
    file: {
      filename: 'financial-report.pdf',
      size: 14.2 * 1024 * 1024,
      media_type: 'application/pdf',
      sha256: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
      download_url: 'http://127.0.0.1:8081/f/file-share-123',
    },
  };

  it('renders file metadata and direct download link', () => {
    const { container } = render(<FileViewer share={sampleShare} />);

    expect(screen.getByText('financial-report.pdf')).toBeInTheDocument();
    expect(screen.getByText('14.2 MiB')).toBeInTheDocument();
    expect(screen.getByText('application/pdf')).toBeInTheDocument();
    expect(screen.getByText(/in \d+ days/)).toBeInTheDocument();

    const downloadLink = screen.getByRole('link', { name: 'Download file' });
    expect(downloadLink).toHaveAttribute('href', 'http://127.0.0.1:8081/f/file-share-123');
    expect(downloadLink).toHaveAttribute('download', 'financial-report.pdf');

    // Strict security check: no media preview elements
    expect(container.querySelector('iframe')).toBeNull();
    expect(container.querySelector('embed')).toBeNull();
    expect(container.querySelector('object')).toBeNull();
    expect(container.querySelector('video')).toBeNull();
    expect(container.querySelector('audio')).toBeNull();
    expect(container.querySelector('img')).toBeNull();
  });

  it('does not render unsafe download URLs or a preview', () => {
    const unsafeUrl = 'javascript:alert(1)';
    const { container } = render(
      <FileViewer share={{ ...sampleShare, file: { ...sampleShare.file!, download_url: unsafeUrl } }} />
    );

    expect(screen.queryByRole('link', { name: 'Download file' })).toBeNull();
    expect(screen.getByText('Download unavailable')).toBeInTheDocument();
    expect(container.innerHTML).not.toContain(unsafeUrl);
    expect(container.querySelector('iframe, embed, object, video, audio, img')).toBeNull();
  });

  it('sanitizes filename preventing traversal display', () => {
    const maliciousShare: ShareMetadata = {
      ...sampleShare,
      file: {
        ...sampleShare.file!,
        filename: '../../../etc/shadow\u0000.txt',
      },
    };

    render(<FileViewer share={maliciousShare} />);

    expect(screen.queryByText(/etc/)).toBeNull();
    expect(screen.getByText('shadow.txt')).toBeInTheDocument();
  });

  it('renders "Never" when expires_at is null', () => {
    const neverExpShare: ShareMetadata = {
      ...sampleShare,
      expires_at: null,
    };

    render(<FileViewer share={neverExpShare} />);
    expect(screen.getByText('Never')).toBeInTheDocument();
  });
});
