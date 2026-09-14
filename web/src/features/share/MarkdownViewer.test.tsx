import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MarkdownViewer } from './MarkdownViewer';

describe('MarkdownViewer', () => {
  it('renders Markdown in Rendered mode by default', () => {
    const md = '# Heading 1\n\n**bold text** and *italic*\n\n- item 1\n- item 2';
    const { container } = render(
      <MarkdownViewer
        shareId="md-share-1"
        content={md}
        isEncrypted={false}
        expiresAt={null}
      />
    );

    expect(screen.getByText('Markdown')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1, name: 'Heading 1' })).toBeInTheDocument();
    expect(screen.getByText('bold text')).toBeInTheDocument();
    expect(screen.getByText('item 1')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Raw' })).toBeInTheDocument();

    const renderedBtn = screen.getByRole('button', { name: 'Rendered' });
    const sourceBtn = screen.getByRole('button', { name: 'Source' });
    expect(renderedBtn).toHaveAttribute('aria-pressed', 'true');
    expect(sourceBtn).toHaveAttribute('aria-pressed', 'false');
  });

  it('switches between Rendered and Source view modes', async () => {
    const user = userEvent.setup();
    const md = '## My Section\n\n```js\nconsole.log(1);\n```';
    render(
      <MarkdownViewer
        shareId="md-share-2"
        content={md}
        isEncrypted={false}
        expiresAt={null}
      />
    );

    // Initial state: rendered
    expect(screen.getByRole('heading', { level: 2, name: 'My Section' })).toBeInTheDocument();

    // Click Source
    const sourceBtn = screen.getByRole('button', { name: 'Source' });
    await user.click(sourceBtn);

    expect(sourceBtn).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByRole('heading', { level: 2 })).not.toBeInTheDocument();

    const sourceContainer = screen.getByLabelText('Markdown source content');
    expect(sourceContainer).toHaveClass('markdown-source-content');
    expect(sourceContainer.textContent).toContain('## My Section');

    // Click Rendered again
    const renderedBtn = screen.getByRole('button', { name: 'Rendered' });
    await user.click(renderedBtn);
    expect(screen.getByRole('heading', { level: 2, name: 'My Section' })).toBeInTheDocument();
  });

  it('strictly prevents <img> elements and remote media embedding', () => {
    const md = 'Here is an image: ![Diagram](https://example.com/leak-ip.png)';
    const { container } = render(
      <MarkdownViewer
        shareId="md-share-3"
        content={md}
        isEncrypted={false}
        expiresAt={null}
      />
    );

    // ZERO <img> tags allowed
    const imgEl = container.querySelector('img');
    expect(imgEl).toBeNull();

    // Renders safe outbound link with [Image: Diagram]
    const fallbackLink = screen.getByRole('link', { name: '[Image: Diagram]' });
    expect(fallbackLink).toHaveAttribute('href', 'https://example.com/leak-ip.png');
    expect(fallbackLink).toHaveAttribute('target', '_blank');
    expect(fallbackLink).toHaveAttribute('rel', 'noopener noreferrer nofollow');
  });

  it('neutralizes unsafe links and javascript: schemes', () => {
    const md = `
[Safe Link](https://example.com)
[Evil JS](javascript:alert('xss'))
[Evil Data](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)
[Relative](/internal/path)
`;
    const { container } = render(
      <MarkdownViewer
        shareId="md-share-4"
        content={md}
        isEncrypted={false}
        expiresAt={null}
      />
    );

    // Safe link is kept
    const safeLink = screen.getByRole('link', { name: 'Safe Link' });
    expect(safeLink).toHaveAttribute('href', 'https://example.com');
    expect(safeLink).toHaveAttribute('target', '_blank');
    expect(safeLink).toHaveAttribute('rel', 'noopener noreferrer nofollow');

    // Dangerous links are NOT rendered as links
    expect(screen.queryByRole('link', { name: 'Evil JS' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Evil Data' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Relative' })).toBeNull();

    // Content is rendered as harmless text
    expect(screen.getByText('Evil JS')).toBeInTheDocument();
    expect(screen.getByText('Evil Data')).toBeInTheDocument();
    expect(screen.getByText('Relative')).toBeInTheDocument();
  });

  it('neutralizes raw HTML and active script tags', () => {
    const md = `
Hello

<script>window.__pwned = true;</script>

<iframe src="https://evil.com"></iframe>

<button onclick="alert(1)">Click me</button>
<style>body { display: none }</style>
<object data="https://evil.com"></object>
<embed src="https://evil.com">
<svg onload="alert(1)"></svg>
<img src="https://evil.com/pixel" onerror="alert(1)">

World
`;
    const { container } = render(
      <MarkdownViewer
        shareId="md-share-5"
        content={md}
        isEncrypted={false}
        expiresAt={null}
      />
    );

    expect(container.querySelector('script')).toBeNull();
    expect(container.querySelector('iframe')).toBeNull();
    expect(container.querySelector('button[onclick]')).toBeNull();
    expect(container.querySelector('style, object, embed, svg, img, [onload], [onerror]')).toBeNull();
    expect(screen.getByText('Hello')).toBeInTheDocument();
    expect(screen.getByText('World')).toBeInTheDocument();
  });

  it('renders GFM task list checkboxes as disabled and read-only', () => {
    const md = `
- [ ] Incomplete task
- [x] Completed task
`;
    const { container } = render(
      <MarkdownViewer
        shareId="md-share-6"
        content={md}
        isEncrypted={false}
        expiresAt={null}
      />
    );

    const checkboxes = container.querySelectorAll('input[type="checkbox"]');
    expect(checkboxes.length).toBe(2);
    expect(checkboxes[0]).toBeDisabled();
    expect(checkboxes[0]).not.toBeChecked();
    expect(checkboxes[1]).toBeDisabled();
    expect(checkboxes[1]).toBeChecked();
  });

  it('omits Raw button for encrypted Markdown', () => {
    render(
      <MarkdownViewer
        shareId="md-share-7"
        content="# Encrypted Note"
        isEncrypted={true}
        expiresAt={null}
      />
    );

    expect(screen.queryByRole('link', { name: 'Raw' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /copy to clipboard/i })).toBeInTheDocument();
  });
});
