import React, { useEffect, useRef } from 'react';

export function Dialog({ title, children, onClose }: { title: string; children: React.ReactNode; onClose: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const focusable = () => ref.current?.querySelector<HTMLElement>('button, input, select, textarea, [href]');
    focusable()?.focus();
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
      if (event.key !== 'Tab' || !ref.current) return;
      const items = [...ref.current.querySelectorAll<HTMLElement>('button, input, select, textarea, [href]')].filter((item) => !item.hasAttribute('disabled'));
      if (!items.length) return;
      const first = items[0], last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', keydown);
    return () => { document.removeEventListener('keydown', keydown); previous?.focus(); };
  }, [onClose]);
  return <div className="dialog-backdrop" role="presentation"><div className="dialog" ref={ref} role="dialog" aria-modal="true" aria-label={title}><h2>{title}</h2>{children}</div></div>;
}
