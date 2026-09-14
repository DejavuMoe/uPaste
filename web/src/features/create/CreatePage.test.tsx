import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router';
import { CreatePage } from './CreatePage';
import { OwnerCapabilityProvider } from '../../app/ownerCapabilities';

describe('CreatePage draft preservation & beforeunload', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  const renderCreatePage = () => {
    return render(
      <MemoryRouter>
        <OwnerCapabilityProvider>
          <CreatePage />
        </OwnerCapabilityProvider>
      </MemoryRouter>,
    );
  };

  it('sets document title to "New share · uPaste"', () => {
    renderCreatePage();
    expect(document.title).toBe('New share · uPaste');
  });

  it('preserves text draft when switching from Text to File and back to Text', () => {
    renderCreatePage();

    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'my draft text that must survive' } });

    // Switch to File tab
    const fileTab = screen.getByRole('tab', { name: /file/i });
    fireEvent.click(fileTab);

    // Text panel is hidden
    const textPanel = document.getElementById('tabpanel-text');
    expect(textPanel).toHaveAttribute('hidden');

    // File panel is visible
    const filePanel = document.getElementById('tabpanel-file');
    expect(filePanel).not.toHaveAttribute('hidden');

    // Switch back to Text tab
    const textTab = screen.getByRole('tab', { name: /text/i });
    fireEvent.click(textTab);

    expect(textPanel).not.toHaveAttribute('hidden');
    expect(textarea).toHaveValue('my draft text that must survive');
  });

  it('preserves selected file when switching from File to Text and back to File', () => {
    renderCreatePage();

    // Switch to File tab
    const fileTab = screen.getByRole('tab', { name: /file/i });
    fireEvent.click(fileTab);

    const validFile = new File(['file contents'], 'preserved-report.pdf', {
      type: 'application/pdf',
    });
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(fileInput, { target: { files: [validFile] } });

    expect(screen.getByText('preserved-report.pdf')).toBeInTheDocument();

    // Switch to Text tab
    const textTab = screen.getByRole('tab', { name: /text/i });
    fireEvent.click(textTab);

    // Switch back to File tab
    fireEvent.click(fileTab);

    expect(screen.getByText('preserved-report.pdf')).toBeInTheDocument();
  });

  it('triggers beforeunload prevention when Text or File is dirty', () => {
    renderCreatePage();

    // 1. Initially empty: beforeunload not prevented
    const emptyEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(emptyEvent);
    expect(emptyEvent.defaultPrevented).toBe(false);

    // 2. Add text draft: beforeunload prevented
    const textarea = screen.getByPlaceholderText(/paste or type content here/i);
    fireEvent.change(textarea, { target: { value: 'unsaved draft' } });

    const dirtyTextEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(dirtyTextEvent);
    expect(dirtyTextEvent.defaultPrevented).toBe(true);

    // Clear text: beforeunload not prevented
    fireEvent.change(textarea, { target: { value: '' } });
    const clearedTextEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(clearedTextEvent);
    expect(clearedTextEvent.defaultPrevented).toBe(false);

    // 3. Switch to File and select file: beforeunload prevented
    const fileTab = screen.getByRole('tab', { name: /file/i });
    fireEvent.click(fileTab);

    const validFile = new File(['data'], 'test.txt');
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(fileInput, { target: { files: [validFile] } });

    const dirtyFileEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(dirtyFileEvent);
    expect(dirtyFileEvent.defaultPrevented).toBe(true);

    // Remove file: beforeunload not prevented
    const removeBtn = screen.getByRole('button', { name: /remove/i });
    fireEvent.click(removeBtn);

    const clearedFileEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(clearedFileEvent);
    expect(clearedFileEvent.defaultPrevented).toBe(false);
  });
});
