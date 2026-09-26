import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import React from 'react';
import { FileCreateForm } from './FileCreateForm';
import * as api from '../../app/api';

describe('FileCreateForm', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders initial dropzone with 64 MiB limit notice', () => {
    render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    expect(screen.getByText(/drop one file here/i)).toBeInTheDocument();
    expect(screen.getByText(/one file, up to 64 mib/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create share/i })).toBeDisabled();
  });

  it('rejects zero-byte files immediately upon selection', () => {
    const createFileSpy = vi.spyOn(api, 'createFile');
    render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    const emptyFile = new File([], 'empty.txt', { type: 'text/plain' });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [emptyFile] } });

    expect(screen.getByRole('alert')).toHaveTextContent(
      'File is empty. Select a file with at least 1 byte.',
    );
    expect(screen.queryByText('empty.txt')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create share/i })).toBeDisabled();
    expect(createFileSpy).not.toHaveBeenCalled();
  });

  it('rejects files larger than 64 MiB immediately upon selection', () => {
    render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    // 64 MiB + 1 byte
    const oversizedFile = new File([''], 'huge.bin');
    Object.defineProperty(oversizedFile, 'size', { value: 67_108_865 });

    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [oversizedFile] } });

    expect(screen.getByRole('alert')).toHaveTextContent(/exceeds maximum size limit of 64 MiB/i);
    expect(screen.queryByText('huge.bin')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create share/i })).toBeDisabled();
  });

  it('handles multi-file drop by selecting only first file and displaying notice', () => {
    render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    const file1 = new File(['first content'], 'first.txt', { type: 'text/plain' });
    const file2 = new File(['second content'], 'second.txt', { type: 'text/plain' });

    const dropzone = screen.getByLabelText(/drop one file here or choose file/i);
    fireEvent.drop(dropzone, {
      dataTransfer: {
        files: [file1, file2],
      },
    });

    expect(
      screen.getByText('Only one file per share is supported. The first file was selected.'),
    ).toBeInTheDocument();
    expect(screen.getByText('first.txt')).toBeInTheDocument();
    expect(screen.queryByText('second.txt')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create share/i })).not.toBeDisabled();

    // Removing file clears the notice
    const removeBtn = screen.getByRole('button', { name: /remove/i });
    fireEvent.click(removeBtn);
    expect(
      screen.queryByText('Only one file per share is supported. The first file was selected.'),
    ).not.toBeInTheDocument();
  });

  it('accepts valid file <= 64 MiB and collapses into file row', () => {
    render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    const validFile = new File(['sample test file content'], 'report.pdf', {
      type: 'application/pdf',
    });

    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [validFile] } });

    // Dropzone collapsed, file row visible
    expect(screen.queryByText(/drop one file here/i)).not.toBeInTheDocument();
    expect(screen.getByText('report.pdf')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /remove/i })).toBeInTheDocument();

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    expect(submitBtn).not.toBeDisabled();
  });

  it('removes file when clicking Remove and restores dropzone', () => {
    render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    const validFile = new File(['content'], 'test.txt');
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [validFile] } });

    const removeBtn = screen.getByRole('button', { name: /remove/i });
    fireEvent.click(removeBtn);

    expect(screen.getByText(/drop one file here/i)).toBeInTheDocument();
    expect(screen.queryByText('test.txt')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create share/i })).toBeDisabled();
  });

  it('renders progress without fabricated percentage when lengthComputable is false', async () => {
    (vi.spyOn(api, 'createFile') as any).mockImplementation(async (_file: any, _expiresAt: any, onProgress: any) => {
      onProgress?.({
        loaded: 1024,
        total: 0,
        lengthComputable: false,
        percent: null,
      });
      // Keep promise pending
      return new Promise(() => {});
    });

    render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    const validFile = new File(['file data'], 'doc.pdf');
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [validFile] } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(document.querySelector('.progress-status-text')).toHaveTextContent('Uploading…');
    });

    // Verify NO percentage or progress bar track
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    expect(screen.queryByText(/%/)).not.toBeInTheDocument();
  });

  it('submits file upload, triggers progress, and calls onSuccess', async () => {
    const onSuccess = vi.fn();
    const createFileSpy = (vi.spyOn(api, 'createFile') as any)
      .mockImplementation(async (file: any, expiresAt: any, onProgress: any) => {
        onProgress?.({
          loaded: 50,
          total: 100,
          lengthComputable: true,
          percent: 50,
        });
        return {
          share: {
            id: 'file-share-999',
            payload_kind: 'FILE',
            privacy_mode: 'STANDARD',
            created_at: '2026-09-14T00:00:00Z',
            updated_at: '2026-09-14T00:00:00Z',
            expires_at: null,
          },
          owner_token: 'up_o1_filetok999',
        };
      });

    render(<FileCreateForm language="en" onSuccess={onSuccess} />);

    const validFile = new File(['file data'], 'doc.pdf');
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [validFile] } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(createFileSpy).toHaveBeenCalledWith(
        validFile,
        null,
        expect.any(Function),
        expect.any(AbortSignal),
      );
    });

    expect(onSuccess).toHaveBeenCalledWith(expect.objectContaining({
      shareId: 'file-share-999',
      ownerToken: 'up_o1_filetok999',
      kind: 'file',
      privacy: 'STANDARD',
    }));
  });

  it('aborts active upload when component unmounts', () => {
    let capturedSignal: AbortSignal | undefined;
    (vi.spyOn(api, 'createFile') as any).mockImplementation(async (_f: any, _e: any, _p: any, signal: AbortSignal) => {
      capturedSignal = signal;
      return new Promise(() => {});
    });

    const { unmount } = render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    const validFile = new File(['file data'], 'doc.pdf');
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [validFile] } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    expect(capturedSignal).toBeDefined();
    expect(capturedSignal?.aborted).toBe(false);

    unmount();

    expect(capturedSignal?.aborted).toBe(true);
  });

  it('displays inline error on upload failure and preserves file selection', async () => {
    vi.spyOn(api, 'createFile').mockRejectedValue(
      new api.ApiError(500, 'server_error', 'Upload failed'),
    );

    render(<FileCreateForm language="en" onSuccess={vi.fn()} />);

    const validFile = new File(['file data'], 'doc.pdf');
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [validFile] } });

    const submitBtn = screen.getByRole('button', { name: /create share/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Upload failed');
    });

    // File selection is preserved so user can retry without re-selecting
    expect(screen.getByText('doc.pdf')).toBeInTheDocument();
  });
});
