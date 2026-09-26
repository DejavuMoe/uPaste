import React, { useState, useRef, useEffect } from 'react';
import { createFile, ApiError } from '../../app/api';
import type { UploadProgress } from '../../app/types';
import { Button } from '../../components/Button';
import { ExpirationField, type ExpirationValue } from '../../components/ExpirationField';
import { ChallengeGate } from '../challenge/ChallengeGate';
import { useDeploymentConfig } from '../../app/config';
import { useLanguage, type Language } from '../../app/locale';
import { createCopy } from './createCopy';

export interface FileCreateSuccessData {
  shareId: string;
  ownerToken: string;
  kind: 'file';
  privacy: 'STANDARD';
  expiration: ExpirationValue['preset'];
}

export interface FileCreateFormProps {
  onSuccess: (data: FileCreateSuccessData) => void;
  onDirtyChange?: (isDirty: boolean) => void;
  onFileSizeChange?: (bytes: number) => void;
  language?: Language;
}

export const MAX_FILE_BYTES = 64 * 1024 * 1024; // 64 MiB = 67,108,864 bytes

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

export const FileCreateForm: React.FC<FileCreateFormProps> = ({ onSuccess, onDirtyChange, onFileSizeChange, language: languageProp }) => {
  const currentLanguage = useLanguage().language;
  const language = languageProp ?? currentLanguage;
  const t = createCopy[language];
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [isDragging, setIsDragging] = useState<boolean>(false);
  const [expiration, setExpiration] = useState<ExpirationValue>({
    preset: 'never',
    resolveExpiresAt: () => null,
  });
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [uploadProgress, setUploadProgress] = useState<UploadProgress | null>(null);
  const [statusText, setStatusText] = useState<string>('');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [noticeMessage, setNoticeMessage] = useState<string | null>(null);
  const { config } = useDeploymentConfig();
  const challengeConfig = config.challenge;
  const requiresChallenge = !!challengeConfig;
  const [challengeToken, setChallengeToken] = useState<string | null>(null);
  const [challengeResetKey, setChallengeResetKey] = useState(0);
  const expirationPolicy = {
    mode: config.deployment_mode,
    defaultSeconds: config.retention?.default_seconds,
    maxSeconds: config.retention?.max_seconds,
  };

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
    onFileSizeChange?.(selectedFile?.size ?? 0);
  }, [selectedFile, onDirtyChange, onFileSizeChange]);

  const handleFileSelection = (file: File | null) => {
    setErrorMessage(null);
    if (!file) {
      setSelectedFile(null);
      return;
    }

    if (file.size === 0) {
      setErrorMessage(t.emptyFile);
      setSelectedFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
      return;
    }

    if (file.size > MAX_FILE_BYTES) {
      setErrorMessage(`${t.largeFile} (${formatFileSize(file.size)})`);
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
        setNoticeMessage(t.manyFiles);
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
    if (requiresChallenge && !challengeToken) {
      setErrorMessage(t.challenge);
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    setStatusText(t.preparing);
    setUploadProgress(null);

    const controller = new AbortController();
    abortControllerRef.current = controller;
    const expiresAt = expiration.resolveExpiresAt();

    try {
      const res = await (requiresChallenge
        ? createFile(selectedFile, expiresAt, challengeToken, (progress) => {
          if (!isMountedRef.current) return;
          setStatusText(t.uploading);
          setUploadProgress(progress);
        }, controller.signal)
        : createFile(selectedFile, expiresAt, (progress) => {
            if (!isMountedRef.current) return;
            setStatusText(t.uploading);
            setUploadProgress(progress);
          }, controller.signal));

      if (!isMountedRef.current) return;
      onSuccess({
        shareId: res.share.id,
        ownerToken: res.owner_token,
        kind: 'file', privacy: 'STANDARD', expiration: expiration.preset,
      });
    } catch (err: any) {
      if (err.name === 'AbortError') return;
      if (!isMountedRef.current) return;

      if (err instanceof ApiError) {
        if (err.status === 429 && err.retryAfterSeconds) {
          setErrorMessage(t.rateLimit(err.retryAfterSeconds));
        } else {
          setErrorMessage(err.message || `Error ${err.status}: ${err.code}`);
        }
      } else if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage(t.uploadFailed);
      }
    } finally {
      if (isMountedRef.current) {
        setIsSubmitting(false);
        setStatusText('');
        if (requiresChallenge) {
          setChallengeToken(null);
          setChallengeResetKey((key) => key + 1);
        }
      }
      abortControllerRef.current = null;
    }
  };

  const hasExpirationError = !!expiration.error;
  const canSubmit = !!selectedFile && !isSubmitting && !hasExpirationError && (!requiresChallenge || challengeToken !== null);

  return (
    <form className="create-form file-create-form" onSubmit={handleSubmit} noValidate>
      <div
        className={`create-file-area ${isDragging ? 'is-dragging' : ''}`}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        aria-label={language === 'zh' ? '拖入一个文件或选择文件' : 'Drop one file here or choose file'}
      >
        <input
          ref={fileInputRef}
          type="file"
          className="sr-only"
          tabIndex={-1}
          disabled={isSubmitting}
          onChange={(event) => {
            setNoticeMessage(null);
            if (event.target.files?.length) handleFileSelection(event.target.files[0]);
          }}
          aria-label={t.chooseFile}
        />
        {!selectedFile ? (
          <div className="create-file-empty">
            <p>{t.dropFile}</p>
            <Button type="button" variant="secondary" disabled={isSubmitting} onClick={() => fileInputRef.current?.click()}>{t.chooseFile}</Button>
            <span>{t.fileLimit}</span>
          </div>
        ) : (
          <div className="create-file-selected">
            <div><strong title={selectedFile.name}>{selectedFile.name}</strong><span>{formatFileSize(selectedFile.size)}</span></div>
            <Button type="button" variant="secondary" disabled={isSubmitting} onClick={handleRemove} aria-label={`${t.remove} ${selectedFile.name}`}>{t.remove}</Button>
          </div>
        )}
      </div>

      {noticeMessage && <p className="create-note" role="status">{noticeMessage}</p>}
      {errorMessage && <p className="create-error" role="alert">{errorMessage}</p>}
      {isSubmitting && uploadProgress && (
        <div className="upload-progress-container" aria-live="polite">
          {uploadProgress.lengthComputable && (
            <div className="progress-bar-track">
              <div className="progress-bar-fill" style={{ width: `${uploadProgress.percent ?? 0}%` }} role="progressbar" aria-valuenow={uploadProgress.percent ?? 0} aria-valuemin={0} aria-valuemax={100} />
            </div>
          )}
          <div className="progress-status-text">
            {uploadProgress.lengthComputable
              ? `${statusText} ${uploadProgress.percent !== null ? `${uploadProgress.percent}%` : ''} (${formatFileSize(uploadProgress.loaded)} / ${formatFileSize(uploadProgress.total)})`
              : statusText}
          </div>
        </div>
      )}
      <div className="create-controls file-mode">
        <div className="create-setting">
          <label htmlFor="file-expiration">{t.expires}</label>
          <ExpirationField id="file-expiration" className="create-expiration" value={expiration.preset} onChange={setExpiration} disabled={isSubmitting} policy={expirationPolicy} language={language} />
        </div>
      </div>
      {requiresChallenge && challengeConfig && <ChallengeGate challenge={challengeConfig} onToken={setChallengeToken} resetKey={challengeResetKey} language={language} />}
      <div className="create-submit">
        <Button type="submit" variant="primary" disabled={!canSubmit} loading={isSubmitting}>{isSubmitting ? statusText : t.create}</Button>
      </div>
    </form>
  );
};
