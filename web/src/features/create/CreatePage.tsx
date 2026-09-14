import React, { useState, useEffect } from 'react';
import { Tabs } from '../../components/Tabs';
import { TextCreateForm, type TextCreateSuccessData } from './TextCreateForm';
import { FileCreateForm, type FileCreateSuccessData } from './FileCreateForm';
import { CreationSuccess } from './CreationSuccess';
import { useDocumentTitle } from '../../app/useDocumentTitle';

type TabKey = 'text' | 'file';

interface SuccessState {
  shareId: string;
  ownerToken: string;
  encryptionKey?: string;
}

export const CreatePage: React.FC = () => {
  useDocumentTitle('New share · uPaste');

  const [activeTab, setActiveTab] = useState<TabKey>('text');
  const [successData, setSuccessData] = useState<SuccessState | null>(null);
  const [isTextDirty, setIsTextDirty] = useState<boolean>(false);
  const [isFileDirty, setIsFileDirty] = useState<boolean>(false);
  const [formInstanceKey, setFormInstanceKey] = useState<number>(0);

  const isDirty = (isTextDirty || isFileDirty) && !successData;

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
  };

  if (successData) {
    return (
      <main className="page-container" id="main-content">
        <CreationSuccess
          shareId={successData.shareId}
          ownerToken={successData.ownerToken}
          encryptionKey={successData.encryptionKey}
          onReset={handleReset}
        />
      </main>
    );
  }

  const tabs = [
    { id: 'text' as const, label: 'Text' },
    { id: 'file' as const, label: 'File' },
  ];

  return (
    <main className="page-container" id="main-content">
      <div className="create-workspace">
        <h1 className="page-title">New share</h1>

        <Tabs
          tabs={tabs}
          activeTab={activeTab}
          onChange={(id) => setActiveTab(id as TabKey)}
          className="create-tabs"
        />

        <div className="tab-panels" key={formInstanceKey}>
          <div
            id="tabpanel-text"
            role="tabpanel"
            aria-labelledby="tab-text"
            hidden={activeTab !== 'text'}
            className="tab-panel"
          >
            <TextCreateForm
              onSuccess={handleSuccess}
              onDirtyChange={setIsTextDirty}
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
              onSuccess={handleSuccess}
              onDirtyChange={setIsFileDirty}
            />
          </div>
        </div>
      </div>
    </main>
  );
};
