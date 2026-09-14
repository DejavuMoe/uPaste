import React, { useState } from 'react';
import { Tabs } from '../../components/Tabs';
import { TextCreateForm, type TextCreateSuccessData } from './TextCreateForm';
import { FileCreateForm, type FileCreateSuccessData } from './FileCreateForm';
import { CreationSuccess } from './CreationSuccess';

type TabKey = 'text' | 'file';

interface SuccessState {
  shareId: string;
  ownerToken: string;
  encryptionKey?: string;
}

export const CreatePage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<TabKey>('text');
  const [successData, setSuccessData] = useState<SuccessState | null>(null);

  const handleSuccess = (data: TextCreateSuccessData | FileCreateSuccessData) => {
    setSuccessData(data);
  };

  const handleReset = () => {
    setSuccessData(null);
    setActiveTab('text');
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

        <div className="tab-content" role="region" aria-label={`${activeTab === 'text' ? 'Text' : 'File'} share creation`}>
          {activeTab === 'text' ? (
            <TextCreateForm onSuccess={handleSuccess} />
          ) : (
            <FileCreateForm onSuccess={handleSuccess} />
          )}
        </div>
      </div>
    </main>
  );
};
