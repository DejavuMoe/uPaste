import React from 'react';
import { useParams, Link } from 'react-router';
import { useDocumentTitle } from '../../app/useDocumentTitle';

export const ShareRoute: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  useDocumentTitle('Share · uPaste');

  return (
    <main className="page-container" id="main-content">
      <div className="placeholder-container">
        <div className="placeholder-header">
          <h1 className="page-title">Share viewer</h1>
          <Link to="/" className="btn btn-secondary">
            New share
          </Link>
        </div>
        <div className="placeholder-card">
          <p className="placeholder-meta">
            <strong>Share ID:</strong> <span className="font-mono">{id}</span>
          </p>
          <p className="placeholder-note">
            Share viewer is not available in this build.
          </p>
        </div>
      </div>
    </main>
  );
};
