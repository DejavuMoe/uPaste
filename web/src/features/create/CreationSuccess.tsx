import React, { useRef, useState } from 'react';
import { useNavigate } from 'react-router';
import { Button } from '../../components/Button';
import { CopyButton } from '../../components/CopyButton';
import { useOwnerCapabilities } from '../../app/ownerCapabilities';
import { useLanguage, type Language } from '../../app/locale';
import { createCopy } from './createCopy';

export interface CreationSuccessProps {
  shareId: string;
  ownerToken: string;
  encryptionKey?: string;
  kind?: 'text' | 'file';
  format?: 'PLAIN' | 'SOURCE' | 'MARKDOWN';
  privacy?: 'STANDARD' | 'ENCRYPTED';
  expiration?: 'never' | '1h' | '1d' | '7d' | '30d' | 'custom';
  language?: Language;
  onReset: () => void;
}

export const CreationSuccess: React.FC<CreationSuccessProps> = ({
  shareId, ownerToken, encryptionKey, kind, format, privacy, expiration, language: languageProp, onReset,
}) => {
  const currentLanguage = useLanguage().language;
  const language = languageProp ?? currentLanguage;
  const t = createCopy[language];
  const navigate = useNavigate();
  const { remember } = useOwnerCapabilities();
  const [tokenRevealed, setTokenRevealed] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);
  const linkRef = useRef<HTMLInputElement>(null);
  const tokenRef = useRef<HTMLInputElement>(null);

  const fragment = encryptionKey ? `#up_e1_${encryptionKey}` : '';
  const sharePath = `/s/${shareId}${fragment}`;
  const origin = typeof window !== 'undefined' && window.location?.origin ? window.location.origin : '';
  const fullShareUrl = `${origin}${sharePath}`;
  const maskedToken = ownerToken.startsWith('up_o1_')
    ? 'up_o1_' + '•'.repeat(Math.max(8, ownerToken.length - 6))
    : '•'.repeat(Math.max(8, ownerToken.length));

  const meta = kind && privacy && expiration ? [
    kind === 'file' ? t.file : format === 'SOURCE' ? t.source : format === 'MARKDOWN' ? t.markdown : t.plain,
    privacy === 'ENCRYPTED' ? t.encrypted : t.standard,
    ({ never: t.never, '1h': t.oneHour, '1d': t.oneDay, '7d': t.sevenDays, '30d': t.thirtyDays, custom: t.custom })[expiration],
  ].join(' · ') : '';

  const handleCopyFailure = (field: 'link' | 'token') => {
    setCopyFailed(true);
    if (field === 'token') setTokenRevealed(true);
    window.setTimeout(() => {
      const input = field === 'token' ? tokenRef.current : linkRef.current;
      input?.focus();
      input?.select();
    }, 0);
  };

  const handleManageShare = () => {
    remember(shareId, ownerToken);
    navigate(`/manage/${shareId}${fragment}`);
  };

  return (
    <div className="creation-success" aria-live="polite">
      {meta && <p className="success-meta">{meta}</p>}
      <div className="success-row">
        <div className="success-label"><label htmlFor="share-link-input">{t.shareLink}</label></div>
        <div className="success-value">
          <input id="share-link-input" ref={linkRef} type="text" readOnly value={fullShareUrl} onFocus={(event) => event.target.select()} aria-label={t.shareLink} />
          <CopyButton textToCopy={fullShareUrl} label={t.copyLink} copiedLabel={t.copied} ariaLabel={t.copyLink} onCopied={() => setCopyFailed(false)} onCopyFailed={() => handleCopyFailure('link')} />
        </div>
      </div>
      <div className="success-row token-row">
        <div className="success-label">
          <label htmlFor="management-token-input">{t.managementToken}</label>
          <p>{t.tokenNote}</p>
        </div>
        <div className="success-value">
          <input id="management-token-input" ref={tokenRef} type="text" readOnly value={tokenRevealed ? ownerToken : maskedToken} onFocus={(event) => event.target.select()} aria-label={t.managementToken} />
          <Button type="button" variant="secondary" onClick={() => setTokenRevealed((previous) => !previous)} aria-label={tokenRevealed ? t.hide : t.reveal}>{tokenRevealed ? t.hide : t.reveal}</Button>
          <CopyButton textToCopy={ownerToken} label={t.copyToken} copiedLabel={t.copied} ariaLabel={t.copyToken} variant="primary" onCopied={() => setCopyFailed(false)} onCopyFailed={() => handleCopyFailure('token')} />
        </div>
      </div>
      <p className="success-status" role="status">{copyFailed ? t.copyFailed : ''}</p>
      <div className="success-actions">
        <Button type="button" variant="secondary" onClick={() => navigate(sharePath)}>{t.openShare}</Button>
        <Button type="button" variant="secondary" onClick={handleManageShare}>{t.manageShare}</Button>
        <Button type="button" variant="ghost" onClick={onReset}>{t.newShare}</Button>
      </div>
    </div>
  );
};
