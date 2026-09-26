import React, { useEffect, useRef, useState } from 'react';
import capWasmUrl from '@cap.js/wasm/browser/cap_wasm_bg.wasm?url';
import type { PublicChallengeConfig } from '../../app/config';
import type { Language } from '../../app/locale';

export type ChallengeStatus = 'loading' | 'ready' | 'verifying' | 'verified' | 'expired' | 'error';

export interface ChallengeGateProps {
  challenge: PublicChallengeConfig;
  onToken: (token: string | null) => void;
  onStatus?: (status: ChallengeStatus) => void;
  resetKey?: number;
  language?: Language;
}

declare global {
  interface Window {
    turnstile?: {
      render: (element: HTMLElement, options: Record<string, unknown>) => string;
      remove: (widgetId: string) => void;
      reset: (widgetId: string) => void;
    };
  }
}

const turnstileScriptURL = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';

function loadTurnstile(): Promise<void> {
  if (window.turnstile) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(`script[src="${turnstileScriptURL}"]`);
    if (existing) {
      existing.addEventListener('load', () => resolve(), { once: true });
      existing.addEventListener('error', () => reject(new Error('turnstile script failed')), { once: true });
      return;
    }
    const script = document.createElement('script');
    script.src = turnstileScriptURL;
    script.async = true;
    script.defer = true;
    script.addEventListener('load', () => resolve(), { once: true });
    script.addEventListener('error', () => reject(new Error('turnstile script failed')), { once: true });
    document.head.appendChild(script);
  });
}

const statusLabels: Record<ChallengeStatus, string> = {
  loading: 'Loading verification…',
  ready: 'Verification required',
  verifying: 'Verifying…',
  verified: 'Verified',
  expired: 'Verification expired — solve again',
  error: 'Temporary verification failure — try again',
};

const zhStatusLabels: Record<ChallengeStatus, string> = {
  loading: '正在加载验证…',
  ready: '需要完成人机验证',
  verifying: '正在验证…',
  verified: '已验证',
  expired: '验证已过期，请重新完成',
  error: '验证暂时失败，请重试',
};

export const ChallengeGate: React.FC<ChallengeGateProps> = ({ challenge, onToken, onStatus, resetKey = 0, language = 'en' }) => {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const cleanupRef = useRef<() => void>(() => {});
  const [status, setStatus] = useState<ChallengeStatus>('loading');

  const updateStatus = (next: ChallengeStatus) => {
    setStatus(next);
    onStatus?.(next);
  };

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    onToken(null);
    updateStatus('loading');
    let disposed = false;

    if (challenge.provider === 'cap') {
      let cap: any;
      void (async () => {
        try {
          // Importing the Cap widget eagerly fetches its default CDN WASM, so
          // it must only load for deployments that actually select Cap.
          (window as any).CAP_CUSTOM_WASM_URL = capWasmUrl;
          const capModule = await import('@cap.js/widget');
          if (disposed) return;
          const nonce = document.querySelector<HTMLMetaElement>('meta[name="upaste-csp-nonce"]')?.content;
          if (nonce) {
            (window as any).CAP_CSS_NONCE = nonce;
            (window as any).CAP_SCRIPT_NONCE = nonce;
          }
          const widget = document.createElement('cap-widget') as any;
          widget.setAttribute('data-cap-api-endpoint', challenge.api_endpoint ?? '');
          widget.setAttribute('required', '');
          container.replaceChildren(widget);
          const Cap = capModule.default;
          cap = new Cap({ apiEndpoint: challenge.api_endpoint, required: true }, widget);
          const handleSolve = (event: Event) => {
            const token = (event as CustomEvent<{ token: string }>).detail?.token;
            if (token) {
              onToken(token);
              updateStatus('verified');
            }
          };
          const handleError = () => {
            onToken(null);
            updateStatus('error');
          };
          const handleReset = () => {
            onToken(null);
            updateStatus('expired');
          };
          cap.addEventListener?.('solve', handleSolve);
          cap.addEventListener?.('error', handleError);
          cap.addEventListener?.('reset', handleReset);
          updateStatus('ready');
          void cap.solve?.().catch(() => updateStatus('error'));
          cleanupRef.current = () => {
            cap?.reset?.();
            cap?.removeEventListener?.('solve', handleSolve);
            cap?.removeEventListener?.('error', handleError);
            cap?.removeEventListener?.('reset', handleReset);
            container.replaceChildren();
          };
        } catch {
          if (!disposed) updateStatus('error');
        }
      })();
    } else {
      let widgetId: string | null = null;
      loadTurnstile()
        .then(() => {
          if (disposed || !window.turnstile) return;
          updateStatus('ready');
          widgetId = window.turnstile.render(container, {
            sitekey: challenge.site_key,
            action: 'create_share',
            theme: 'auto',
            callback: (token: string) => {
              onToken(token);
              updateStatus('verified');
            },
            'error-callback': () => {
              onToken(null);
              updateStatus('error');
            },
            'expired-callback': () => {
              onToken(null);
              updateStatus('expired');
            },
          });
        })
        .catch(() => updateStatus('error'));
      cleanupRef.current = () => {
        if (widgetId && window.turnstile) window.turnstile.remove(widgetId);
        container.replaceChildren();
      };
    }

    return () => {
      disposed = true;
      cleanupRef.current();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [challenge.provider, challenge.site_key, challenge.api_endpoint, resetKey]);

  return (
    <div className="challenge-gate" role="group" aria-label={language === 'zh' ? '人机验证' : 'Human verification'}>
      <div ref={containerRef} className="challenge-widget" />
      <p className="challenge-status" role="status" aria-live="polite">
        {language === 'zh' ? zhStatusLabels[status] : statusLabels[status]}
      </p>
    </div>
  );
};
