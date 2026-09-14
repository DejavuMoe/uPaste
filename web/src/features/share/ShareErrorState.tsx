import React from 'react';
import { Link } from 'react-router';

export type ShareErrorType =
  | 'not_found'
  | 'expired'
  | 'rate_limited'
  | 'network_error'
  | 'server_error'
  | 'missing_key'
  | 'decrypt_error';

export interface ShareErrorStateProps {
  type: ShareErrorType;
  retryAfterSeconds?: number;
  onRetry?: () => void;
}

export const ShareErrorState: React.FC<ShareErrorStateProps> = ({
  type,
  retryAfterSeconds,
  onRetry,
}) => {
  let title: string;
  let message: string;
  let icon: string | null = null;
  let action: React.ReactNode;

  switch (type) {
    case 'missing_key':
      title = 'Decryption key missing';
      message =
        'This encrypted share cannot be read without the complete link containing the decryption key.';
      icon = '🔒';
      action = (
        <Link to="/" className="btn btn-primary">
          New share
        </Link>
      );
      break;

    case 'decrypt_error':
      title = 'Unable to decrypt this share';
      message =
        'The link may be incomplete, or the encrypted data may have been modified.';
      icon = '⚠️';
      action = (
        <Link to="/" className="btn btn-primary">
          New share
        </Link>
      );
      break;

    case 'not_found':
      title = 'Share not found';
      message = 'The link may be incorrect, or the share may have been deleted.';
      action = (
        <Link to="/" className="btn btn-primary">
          Create new share
        </Link>
      );
      break;

    case 'expired':
      title = 'This share has expired';
      message = 'Expired shares are no longer accessible and cannot be recovered.';
      action = (
        <Link to="/" className="btn btn-primary">
          Create new share
        </Link>
      );
      break;

    case 'rate_limited': {
      const waitTime = retryAfterSeconds ? `${retryAfterSeconds} seconds` : 'a few moments';
      title = 'Too many requests';
      message = `Please wait approximately ${waitTime} before trying again.`;
      action = onRetry ? (
        <button type="button" className="btn btn-primary" onClick={onRetry}>
          Try again
        </button>
      ) : (
        <Link to="/" className="btn btn-primary">
          New share
        </Link>
      );
      break;
    }

    case 'network_error':
      title = 'Could not reach server';
      message = 'Network connection unavailable or server unreachable.';
      action = onRetry ? (
        <button type="button" className="btn btn-primary" onClick={onRetry}>
          Try again
        </button>
      ) : (
        <Link to="/" className="btn btn-primary">
          New share
        </Link>
      );
      break;

    case 'server_error':
    default:
      title = 'Service error';
      message = 'An unexpected server error occurred. Please try again.';
      action = onRetry ? (
        <button type="button" className="btn btn-primary" onClick={onRetry}>
          Try again
        </button>
      ) : (
        <Link to="/" className="btn btn-primary">
          New share
        </Link>
      );
      break;
  }

  return (
    <article className="viewer-card error-state-card" role="alert">
      <h2 className="error-state-title">
        {icon && <span className="error-state-icon" aria-hidden="true">{icon} </span>}
        {title}
      </h2>
      <p className="error-state-message">{message}</p>
      <div className="error-state-actions">{action}</div>
    </article>
  );
};
