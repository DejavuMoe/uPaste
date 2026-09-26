import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { CreatePage } from './CreatePage';
import { OwnerCapabilityProvider } from '../../app/ownerCapabilities';
import { LanguageProvider, type Language } from '../../app/locale';
import { ThemeProvider } from '../../app/theme';
import { AppHeader } from '../../components/AppHeader';

describe('CreatePage draft preservation & beforeunload', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  const renderCreatePage = (initialEntries = ['/'], language: Language = 'en') => {
    const router = createMemoryRouter([
      { path: '/', element: <><AppHeader /><CreatePage /></> },
      { path: '/s/previous', element: <p>Previous share</p> },
    ], { initialEntries });
    render(<LanguageProvider><ThemeProvider><OwnerCapabilityProvider><RouterProvider router={router} /></OwnerCapabilityProvider></ThemeProvider></LanguageProvider>);
    if (language === 'en') fireEvent.click(screen.getByRole('button', { name: 'EN' }));
    return router;
  };

  it('switches Chinese and English without losing the text draft', () => {
    renderCreatePage(['/'], 'zh');
    expect(screen.getByRole('heading', { name: '新建分享' })).toBeInTheDocument();
    const textarea = screen.getByRole('textbox', { name: '内容' });
    fireEvent.change(textarea, { target: { value: '保留这段草稿' } });

    fireEvent.click(screen.getByRole('button', { name: 'EN' }));
    expect(screen.getByRole('heading', { name: 'New share' })).toBeInTheDocument();
    expect(textarea).toHaveValue('保留这段草稿');

    fireEvent.click(screen.getByRole('button', { name: '中文' }));
    expect(screen.getByRole('heading', { name: '新建分享' })).toBeInTheDocument();
    expect(textarea).toHaveValue('保留这段草稿');
  });

  it('sets document title to "New share · uPaste"', () => {
    renderCreatePage();
    expect(document.title).toBe('New share · uPaste');
  });

  it('preserves text draft when switching from Text to File and back to Text', () => {
    renderCreatePage();

    const textarea = screen.getByRole('textbox', { name: 'Content' });
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
    const textarea = screen.getByRole('textbox', { name: 'Content' });
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

  it('keeps both drafts and the active tab when Back is canceled, then discards them on confirmation', async () => {
    const router = renderCreatePage(['/s/previous', '/']);
    const textarea = screen.getByLabelText('Content');
    fireEvent.change(textarea, { target: { value: 'keep this text' } });
    fireEvent.click(screen.getByRole('tab', { name: 'File' }));
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(fileInput, { target: { files: [new File(['data'], 'keep-this.txt')] } });

    await act(async () => { await router.navigate(-1); });
    expect(screen.getByRole('dialog', { name: 'You have unsaved changes.' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Keep editing' }));
    expect(screen.getByRole('tab', { name: 'File' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('keep-this.txt')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: 'Text' }));
    expect(textarea).toHaveValue('keep this text');

    await act(async () => { await router.navigate(-1); });
    fireEvent.click(screen.getByRole('button', { name: 'Discard and leave' }));
    expect(await screen.findByText('Previous share')).toBeInTheDocument();
  });
});
