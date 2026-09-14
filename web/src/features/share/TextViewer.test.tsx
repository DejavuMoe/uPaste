import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TextViewer } from './TextViewer';

describe('TextViewer', () => {
  it('renders PLAIN text with metadata and content', () => {
    const rawContent = `line 1
  line 2 with spaces
\tline 3 with tab`;

    render(
      <TextViewer
        shareId="plain-share-123"
        format="PLAIN"
        content={rawContent}
        isEncrypted={false}
        expiresAt={null}
      />
    );

    expect(screen.getByText('Plain text')).toBeInTheDocument();
    expect(screen.getByText('Never expires')).toBeInTheDocument();
    expect(screen.getByText(/B|KB/)).toBeInTheDocument();

    const contentEl = screen.getByText(/line 1/);
    expect(contentEl).toHaveClass('plain-text-content');
    expect(contentEl.textContent).toBe(rawContent);

    // Raw button should exist for standard text
    const rawLink = screen.getByRole('link', { name: 'Raw' });
    expect(rawLink).toHaveAttribute('href', '/raw/plain-share-123');

    // Copy button should exist
    expect(screen.getByRole('button', { name: /copy to clipboard/i })).toBeInTheDocument();
  });

  it('omits Raw button for encrypted text', () => {
    render(
      <TextViewer
        shareId="enc-share-123"
        format="PLAIN"
        content="decrypted secret text"
        isEncrypted={true}
        expiresAt={null}
      />
    );

    expect(screen.queryByRole('link', { name: 'Raw' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /copy to clipboard/i })).toBeInTheDocument();
  });

  it('renders SOURCE text with wrap lines control and monospace class', async () => {
    const user = userEvent.setup();
    render(
      <TextViewer
        shareId="src-share-123"
        format="SOURCE"
        content="const x = 42;"
        isEncrypted={false}
        expiresAt="2026-09-20T12:00:00Z"
      />
    );

    expect(screen.getByText('Source')).toBeInTheDocument();

    const codeContainer = screen.getByLabelText('Source code content');
    expect(codeContainer).toHaveClass('source-code-content');
    expect(codeContainer).toHaveClass('no-wrap');

    const wrapBtn = screen.getByRole('button', { name: 'Wrap lines' });
    expect(wrapBtn).toHaveAttribute('aria-pressed', 'false');

    // Click to toggle wrap lines
    await user.click(wrapBtn);
    expect(wrapBtn).toHaveAttribute('aria-pressed', 'true');
    expect(codeContainer).toHaveClass('wrap-lines');
    expect(codeContainer).not.toHaveClass('no-wrap');

    // Click again to toggle back
    await user.click(wrapBtn);
    expect(wrapBtn).toHaveAttribute('aria-pressed', 'false');
    expect(codeContainer).toHaveClass('no-wrap');
  });

  it('copies content when Copy button is clicked', async () => {
    const user = userEvent.setup();
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: writeTextMock },
      configurable: true,
      writable: true,
    });

    render(
      <TextViewer
        shareId="copy-share-123"
        format="PLAIN"
        content="Exact content to copy"
        isEncrypted={false}
        expiresAt={null}
      />
    );

    const copyBtn = screen.getByRole('button', { name: /copy to clipboard/i });
    await user.click(copyBtn);

    expect(writeTextMock).toHaveBeenCalledWith('Exact content to copy');
  });
});
