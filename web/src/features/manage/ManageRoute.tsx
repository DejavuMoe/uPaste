import React, { useState } from 'react';
import { useParams, useLocation, Link } from 'react-router';
import { useOwnerCapabilities } from '../../app/ownerCapabilities';
import { Button } from '../../components/Button';

export const ManageRoute: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const { get, remember } = useOwnerCapabilities();
  const tokenInMemory = id ? get(id) : undefined;

  const [inputToken, setInputToken] = useState('');

  const handleManualSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (id && inputToken.trim()) {
      remember(id, inputToken.trim());
      setInputToken('');
    }
  };

  return (
    <main className="page-container" id="main-content">
      <div className="placeholder-container">
        <div className="placeholder-header">
          <h1 className="page-title">Manage share</h1>
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

          <div className="capability-status-box">
            {tokenInMemory ? (
              <div className="capability-present" role="status">
                <span className="status-indicator-success" aria-hidden="true">✓ </span>
                <strong>In-memory owner capability active</strong>
                <p className="font-mono text-sm">
                  Token: {tokenInMemory.slice(0, 6)}••••••••••••••••••••••••••••••
                </p>
                <p className="text-sm text-secondary">
                  Capability retained securely in volatile process memory.
                </p>
              </div>
            ) : (
              <div className="capability-missing" role="status">
                <span className="status-indicator-warning" aria-hidden="true">⚠️ </span>
                <strong>No owner capability in memory</strong>
                <p className="text-sm text-secondary">
                  Enter your management token below to authorize operations:
                </p>
                <form onSubmit={handleManualSubmit} className="manual-token-form">
                  <input
                    type="password"
                    className="form-input font-mono"
                    placeholder="up_o1_..."
                    value={inputToken}
                    onChange={(e) => setInputToken(e.target.value)}
                    aria-label="Management token"
                  />
                  <Button type="submit" variant="secondary" disabled={!inputToken.trim()}>
                    Set capability
                  </Button>
                </form>
              </div>
            )}
          </div>

          <p className="placeholder-note">
            Full owner-management editor and mutation flows are scheduled for Phase 7C.
          </p>
        </div>
      </div>
    </main>
  );
};
