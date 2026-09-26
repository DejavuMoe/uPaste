import React, { useState, useEffect, useRef } from 'react';
import { useBlocker } from 'react-router';
import { Tabs } from '../../components/Tabs';
import { Button } from '../../components/Button';
import { Dialog } from '../../components/Dialog';
import { TextCreateForm, type TextCreateSuccessData } from './TextCreateForm';
import { FileCreateForm, type FileCreateSuccessData } from './FileCreateForm';
import { CreationSuccess } from './CreationSuccess';
import { useDocumentTitle } from '../../app/useDocumentTitle';
import { useLanguage } from '../../app/locale';
import { createCopy } from './createCopy';
import './createPage.css';

type TabKey = 'text' | 'file';

interface SuccessState {
  shareId: string;
  ownerToken: string;
  encryptionKey?: string;
  kind: 'text' | 'file';
  format?: 'PLAIN' | 'SOURCE' | 'MARKDOWN';
  privacy: 'STANDARD' | 'ENCRYPTED';
  expiration: 'never' | '1h' | '1d' | '7d' | '30d' | 'custom';
}

export const CreatePage: React.FC = () => {
  const { language } = useLanguage();
  const t = createCopy[language];

  const [activeTab, setActiveTab] = useState<TabKey>('text');
  const [successData, setSuccessData] = useState<SuccessState | null>(null);
  const [isTextDirty, setIsTextDirty] = useState<boolean>(false);
  const [isFileDirty, setIsFileDirty] = useState<boolean>(false);
  const [formInstanceKey, setFormInstanceKey] = useState<number>(0);
  const [textBytes, setTextBytes] = useState(0);
  const [fileBytes, setFileBytes] = useState(0);
  const headingRef = useRef<HTMLHeadingElement>(null);
  useDocumentTitle(`${successData ? t.created : t.newShare} · uPaste`);

  useEffect(() => {
    if (successData) headingRef.current?.focus();
  }, [successData]);

  const isDirty = (isTextDirty || isFileDirty) && !successData;
  const blocker = useBlocker(isDirty);

  // beforeunload protection for unsaved draft content
  useEffect(() => {
    if (!isDirty) return;

    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, [isDirty]);

  const handleSuccess = (data: TextCreateSuccessData | FileCreateSuccessData) => {
    setSuccessData(data);
    setIsTextDirty(false);
    setIsFileDirty(false);
  };

  const handleReset = () => {
    setSuccessData(null);
    setActiveTab('text');
    setIsTextDirty(false);
    setIsFileDirty(false);
    setFormInstanceKey((prev) => prev + 1);
    setTextBytes(0);
    setFileBytes(0);
  };

  if (successData) {
    return (
      <main className="page-container create-page" id="main-content">
        <h1 className="create-heading" ref={headingRef} tabIndex={-1}>{t.created}</h1>
        <CreationSuccess
          language={language}
          shareId={successData.shareId}
          ownerToken={successData.ownerToken}
          encryptionKey={successData.encryptionKey}
          kind={successData.kind}
          format={successData.format}
          privacy={successData.privacy}
          expiration={successData.expiration}
          onReset={handleReset}
        />
      </main>
    );
  }

  const tabs = [
    { id: 'text' as const, label: t.text },
    { id: 'file' as const, label: t.file },
  ];
  const size = activeTab === 'text'
    ? `${textBytes < 1024 ? `${textBytes} B` : `${(textBytes / 1024).toFixed(1)} KiB`} / 1 MiB`
    : `${fileBytes < 1024 ? `${fileBytes} B` : fileBytes < 1024 * 1024 ? `${(fileBytes / 1024).toFixed(1)} KiB` : `${(fileBytes / (1024 * 1024)).toFixed(1)} MiB`} / 64 MiB`;

  return (
    <main className="page-container create-page" id="main-content">
      <div className="create-workspace">
        <h1 className="create-heading" ref={headingRef}>{t.newShare}</h1>

        <div className="create-mode-row">
          <Tabs
            tabs={tabs}
            activeTab={activeTab}
            onChange={(id) => setActiveTab(id as TabKey)}
            className="create-tabs"
            label={t.contentType}
          />
          <span className="create-size" id="create-content-size" aria-live="polite">{size}</span>
        </div>

        <div className="tab-panels" key={formInstanceKey}>
          <div
            id="tabpanel-text"
            role="tabpanel"
            aria-labelledby="tab-text"
            hidden={activeTab !== 'text'}
            className="tab-panel"
          >
            <TextCreateForm
              language={language}
              onSuccess={handleSuccess}
              onDirtyChange={setIsTextDirty}
              onByteCountChange={setTextBytes}
            />
          </div>

          <div
            id="tabpanel-file"
            role="tabpanel"
            aria-labelledby="tab-file"
            hidden={activeTab !== 'file'}
            className="tab-panel"
          >
            <FileCreateForm
              language={language}
              onSuccess={handleSuccess}
              onDirtyChange={setIsFileDirty}
              onFileSizeChange={setFileBytes}
            />
          </div>
        </div>
      </div>
      {blocker.state === 'blocked' && (
        <Dialog title={t.unsavedTitle} onClose={() => blocker.reset?.()}>
          <p>{t.unsavedQuestion}</p>
          <Button variant="secondary" onClick={() => blocker.reset?.()}>{t.keepEditing}</Button>
          <Button variant="danger" onClick={() => blocker.proceed?.()}>{t.discard}</Button>
        </Dialog>
      )}
    </main>
  );
};
