import React, { useState, useRef, useEffect } from 'react';
import { createFile, ApiError } from '../../app/api';
import type { UploadProgress } from '../../app/types';
import { Button } from '../../components/Button';
import { ExpirationField, type ExpirationValue } from '../../components/ExpirationField';

export interface FileCreateSuccessData {
  shareId: string;
  ownerToken: string;
}

export interface FileCreateFormProps {
  onSuccess: (data: FileCreateSuccessData) => void;
  onDirtyChange?: (isDirty: boolean) => void;
}

export const MAX_FILE_BYTES = 64 * 1024 * 1024; // 64 MiB = 67,108,864 bytes

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

export const FileCreateForm: React.FC<FileCreateFormProps> = ({ onSuccess, onDirtyChange }) => {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [isDragging, setIsDragging] = useState<boolean>(false);
  const [expiration, setExpiration] = useState<ExpirationValue>({
    preset: 'never',
    resolveExpiresAt: () => null,
  });
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [uploadProgress, setUploadProgress] = useState<UploadProgress | null>(null);
  const [statusText, setStatusText] = useState<string>('Create share');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [noticeMessage, setNoticeMessage] = useState<string | null>(null);

  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const isMountedRef = useRef<boolean>(true);

  // Unmount cleanup
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, []);

  // Report dirty state
  useEffect(() => {
    onDirtyChange?.(selectedFile !== null);
  }, [selectedFile, onDirtyChange]);

  const handleFileSelection = (file: File | null) => {
    setErrorMessage(null);
    if (!file) {
      setSelectedFile(null);
      return;
    }

    if (file.size === 0) {
      setErrorMessage('File is empty. Select a file with at least 1 byte.');
      setSelectedFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
      return;
    }

    if (file.size > MAX_FILE_BYTES) {
      setErrorMessage(`File exceeds maximum size limit of 64 MiB (${formatFileSize(file.size)}).`);
      setSelectedFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
      return;
    }

    setSelectedFile(file);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (!isSubmitting) {
      setIsDragging(true);
    }
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);

    if (isSubmitting) return;

    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      if (e.dataTransfer.files.length > 1) {
        setNoticeMessage('Only one file per share is supported. The first file was selected.');
      } else {
        setNoticeMessage(null);
      }
      handleFileSelection(e.dataTransfer.files[0]);
    }
  };

  const handleRemove = () => {
    setSelectedFile(null);
    setErrorMessage(null);
    setNoticeMessage(null);
    setUploadProgress(null);
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedFile || isSubmitting || !!expiration.error) return;

    setIsSubmitting(true);
    setErrorMessage(null);
    setStatusText('Preparing…');
    setUploadProgress(null);

    const controller = new AbortController();
    abortControllerRef.current = controller;
    const expiresAt = expiration.resolveExpiresAt();

    try {
      const res = await createFile(
        selectedFile,
        expiresAt,
        (progress) => {
          if (!isMountedRef.current) return;
          setStatusText('Uploading…');
          setUploadProgress(progress);
        },
        controller.signal,
      );

      if (!isMountedRef.current) return;
      onSuccess({
        shareId: res.share.id,
        ownerToken: res.owner_token,
      });
    } catch (err: any) {
      if (err.name === 'AbortError') return;
      if (!isMountedRef.current) return;

      if (err instanceof ApiError) {
        if (err.status === 429 && err.retryAfterSeconds) {
          setErrorMessage(
            `Rate limit exceeded. Please wait ${err.retryAfterSeconds} seconds before trying again.`,
          );
        } else {
          setErrorMessage(err.message || `Error ${err.status}: ${err.code}`);
        }
      } else if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage('Failed to upload file. Please try again.');
      }
    } finally {
      if (isMountedRef.current) {
        setIsSubmitting(false);
        setStatusText('Create share');
      }
      abortControllerRef.current = null;
    }
  };

  const hasExpirationError = !!expiration.error;
  const canSubmit = !!selectedFile && !isSubmitting && !hasExpirationError;

  return (
    <form className="create-form file-create-form" onSubmit={handleSubmit} noValidate>
      {noticeMessage && (
        <div className="form-notice-banner" role="status">
          {noticeMessage}
        </div>
      )}

      {errorMessage && (
        <div className="form-error-banner" role="alert">
          {errorMessage}
        </div>
      )}

      {!selectedFile ? (
        <div
          className={`dropzone ${isDragging ? 'dropzone-active' : ''}`}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          aria-label="Drop one file here or choose file"
        >
          <input
            ref={fileInputRef}
            type="file"
            className="sr-only"
            tabIndex={-1}
            disabled={isSubmitting}
            onChange={(e) => {
              setNoticeMessage(null);
              if (e.target.files && e.target.files.length > 0) {
                handleFileSelection(e.target.files[0]);
              }
            }}
            aria-label="File upload"
          />
          <div className="dropzone-content">
            <p className="dropzone-prompt">Drop one file here</p>
            <p className="dropzone-subprompt">or</p>
            <Button
              type="button"
              variant="secondary"
              disabled={isSubmitting}
              onClick={() => {
                fileInputRef.current?.click();
              }}
              aria-label="Choose file"
            >
              Choose file
            </Button>
            <p className="dropzone-limit">Maximum size: 64 MiB</p>
          </div>
        </div>
      ) : (
        <div className="file-selected-row">
          <div className="file-info">
            <span className="file-icon" aria-hidden="true">📄</span>
            <span className="file-name" title={selectedFile.name}>{selectedFile.name}</span>
            <span className="file-size">{formatFileSize(selectedFile.size)}</span>
          </div>
          <Button
            type="button"
            variant="ghost"
            disabled={isSubmitting}
            onClick={handleRemove}
            aria-label={`Remove selected file ${selectedFile.name}`}
          >
            Remove
          </Button>
        </div>
      )}

      {isSubmitting && uploadProgress && (
        <div className="upload-progress-container" aria-live="polite">
          {uploadProgress.lengthComputable && (
            <div className="progress-bar-track">
              <div
                className="progress-bar-fill"
                style={{ width: `${uploadProgress.percent ?? 0}%` }}
                role="progressbar"
                aria-valuenow={uploadProgress.percent ?? 0}
                aria-valuemin={0}
                aria-valuemax={100}
              />
            </div>
          )}
          <div className="progress-status-text">
            {uploadProgress.lengthComputable
              ? `${statusText} ${uploadProgress.percent !== null ? `${uploadProgress.percent}%` : ''} (${formatFileSize(uploadProgress.loaded)} / ${formatFileSize(uploadProgress.total)})`
              : statusText}
          </div>
        </div>
      )}

      <div className="file-form-footer">
        <div className="form-field-group">
          <label htmlFor="file-expiration" className="form-label">
            Expires
          </label>
          <ExpirationField
            id="file-expiration"
            value={expiration.preset}
            onChange={setExpiration}
            disabled={isSubmitting}
          />
        </div>

        <Button
          type="submit"
          variant="primary"
          disabled={!canSubmit}
          loading={isSubmitting}
        >
          {isSubmitting ? statusText : 'Create share'}
        </Button>
      </div>
    </form>
  );
};
