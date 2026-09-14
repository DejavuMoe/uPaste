import React, { useState } from 'react';
import { useNavigate } from 'react-router';
import { Button } from '../../components/Button';
import { CopyButton } from '../../components/CopyButton';
import { useOwnerCapabilities } from '../../app/ownerCapabilities';

export interface CreationSuccessProps {
  shareId: string;
  ownerToken: string;
  encryptionKey?: string; // base64url key without prefix if encrypted text
  onReset: () => void;
}

export const CreationSuccess: React.FC<CreationSuccessProps> = ({
  shareId,
  ownerToken,
  encryptionKey,
  onReset,
}) => {
  const navigate = useNavigate();
  const { remember } = useOwnerCapabilities();
  const [tokenRevealed, setTokenRevealed] = useState(false);

  // Build the full share link URL
  const fragment = encryptionKey ? `#up_e1_${encryptionKey}` : '';
  const sharePath = `/s/${shareId}${fragment}`;
  const origin = typeof window !== 'undefined' && window.location?.origin ? window.location.origin : '';
  const fullShareUrl = `${origin}${sharePath}`;

  // Mask ownerToken: keep prefix 'up_o1_' and replace the remainder with bullets
  const maskedToken = ownerToken.startsWith('up_o1_')
    ? 'up_o1_' + '•'.repeat(Math.max(8, ownerToken.length - 6))
    : '•'.repeat(Math.max(8, ownerToken.length));

  const displayedToken = tokenRevealed ? ownerToken : maskedToken;

  const handleOpenShare = () => {
    navigate(sharePath);
  };

  const handleManageShare = () => {
    // Retain capability in volatile memory only
    remember(shareId, ownerToken);
    navigate(`/manage/${shareId}${fragment}`);
  };

  return (
    <div className="creation-success" aria-live="polite">
      <h2 className="creation-success-title">Share created</h2>

      <div className="success-section">
        <label htmlFor="share-link-input" className="form-label">
          Share link
        </label>
        <div className="input-with-actions">
          <input
            id="share-link-input"
            type="text"
            readOnly
            value={fullShareUrl}
            className="form-input read-only-input"
            onFocus={(e) => e.target.select()}
            aria-label="Share link"
          />
          <CopyButton textToCopy={fullShareUrl} label="Copy link" />
        </div>
      </div>

      <div className="success-section">
        <label htmlFor="management-token-input" className="form-label">
          Management token
        </label>
        <div className="input-with-actions">
          <input
            id="management-token-input"
            type="text"
            readOnly
            value={displayedToken}
            className="form-input read-only-input font-mono"
            onFocus={(e) => e.target.select()}
            aria-label="Management token"
          />
          <Button
            type="button"
            variant="secondary"
            onClick={() => setTokenRevealed((prev) => !prev)}
            aria-label={tokenRevealed ? 'Hide management token' : 'Reveal management token'}
          >
            {tokenRevealed ? 'Hide' : 'Reveal'}
          </Button>
          <CopyButton textToCopy={ownerToken} label="Copy token" />
        </div>
        <p className="token-warning">
          <span aria-hidden="true">⚠️ </span>
          Save this token now. It is shown once and is required to modify or delete this share. uPaste does not store it.
        </p>
      </div>

      <div className="success-actions">
        <Button type="button" variant="primary" onClick={handleOpenShare}>
          Open share
        </Button>
        <Button type="button" variant="secondary" onClick={handleManageShare}>
          Manage share
        </Button>
        <Button type="button" variant="ghost" onClick={onReset}>
          New share
        </Button>
      </div>
    </div>
  );
};
