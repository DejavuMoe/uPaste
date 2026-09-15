import React, { useEffect, useId, useRef } from 'react';
import { createPortal } from 'react-dom';

const selector = 'button:not([disabled]):not([hidden]):not([tabindex="-1"]), [href]:not([hidden]):not([tabindex="-1"]), input:not([disabled]):not([hidden]):not([tabindex="-1"]), select:not([disabled]):not([hidden]):not([tabindex="-1"]), textarea:not([disabled]):not([hidden]):not([tabindex="-1"]), [tabindex]:not([tabindex="-1"])';

export function Dialog({ title, children, onClose }: { title: string; children: React.ReactNode; onClose: () => void }) {
  const ref = useRef<HTMLDivElement>(null); const titleId = useId();
  useEffect(() => {
    const dialog = ref.current!; const previous = document.activeElement as HTMLElement | null;
    const items = () => [...dialog.querySelectorAll<HTMLElement>(selector)];
    const focusFirst = () => (items()[0] || dialog).focus();
    const root = document.getElementById('root'); const oldInert = root?.inert; const hadHidden = root?.hasAttribute('aria-hidden'); const oldHidden = root?.getAttribute('aria-hidden');
    if (root) { root.inert = true; root.setAttribute('aria-hidden', 'true'); }
    focusFirst();
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); onClose(); return; }
      if (event.key !== 'Tab') return;
      const list = items(); if (!list.length) { event.preventDefault(); dialog.focus(); return; }
      const first = list[0], last = list[list.length - 1];
      if (!dialog.contains(document.activeElement)) { event.preventDefault(); focusFirst(); }
      else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    const focusin = (event: FocusEvent) => { if (!dialog.contains(event.target as Node)) focusFirst(); };
    document.addEventListener('keydown', keydown); document.addEventListener('focusin', focusin);
    return () => { document.removeEventListener('keydown', keydown); document.removeEventListener('focusin', focusin); if (root) { root.inert = oldInert!; if (hadHidden) root.setAttribute('aria-hidden', oldHidden!); else root.removeAttribute('aria-hidden'); } if (previous?.isConnected) previous.focus(); };
  }, [onClose]);
  return createPortal(<div className="dialog-backdrop" role="presentation"><div className="dialog" ref={ref} role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1}><h2 id={titleId}>{title}</h2>{children}</div></div>, document.body);
}
