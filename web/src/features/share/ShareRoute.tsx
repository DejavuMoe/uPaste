import React from 'react';
import { useParams, useLocation, Link } from 'react-router';

export const ShareRoute: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();

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
          {location.hash && (
            <p className="placeholder-meta">
              <strong>Key fragment:</strong> <span className="font-mono">{location.hash}</span>
            </p>
          )}
          <p className="placeholder-note">
            Full Share viewer implementation is scheduled for Phase 7B.
          </p>
        </div>
      </div>
    </main>
  );
};
