import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Dialog } from './Dialog';

afterEach(() => { cleanup(); document.getElementById('root')?.remove(); document.body.innerHTML = ''; });
const modal = (children: React.ReactNode = <><button>Cancel</button><button>Delete permanently</button></>) => <Dialog title="Delete this share?" onClose={vi.fn()}>{children}</Dialog>;

describe('Dialog', () => {
  it('has semantic labelled modal dialog', () => { render(modal()); const dialog = screen.getByRole('dialog', { name: 'Delete this share?' }); expect(dialog).toHaveAttribute('aria-modal', 'true'); expect(dialog.getAttribute('aria-labelledby')).toBeTruthy(); });
  it('portals outside render container', () => { const { container } = render(modal()); expect(container.contains(screen.getByRole('dialog'))).toBe(false); expect(document.body.contains(screen.getByRole('dialog'))).toBe(true); });
  it('makes root inert and hidden', () => { const root = document.body.appendChild(document.createElement('div')); root.id = 'root'; render(modal()); expect(root.inert).toBe(true); expect(root).toHaveAttribute('aria-hidden', 'true'); });
  it('restores root state exactly', () => { const root = document.body.appendChild(document.createElement('div')); root.id = 'root'; root.inert = true; root.setAttribute('aria-hidden', 'old'); const { unmount } = render(modal()); unmount(); expect(root.inert).toBe(true); expect(root).toHaveAttribute('aria-hidden', 'old'); });
  it('restores absent root aria-hidden', () => { const root = document.body.appendChild(document.createElement('div')); root.id = 'root'; const { unmount } = render(modal()); unmount(); expect(root.inert).toBeUndefined(); expect(root).not.toHaveAttribute('aria-hidden'); });
  it('focuses first safe control', () => { render(modal()); expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus(); });
  it('wraps forward tab', async () => { const user = userEvent.setup(); render(modal()); screen.getByRole('button', { name: 'Delete permanently' }).focus(); await user.tab(); expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus(); });
  it('wraps reverse tab', async () => { const user = userEvent.setup(); render(modal()); await user.keyboard('{Shift>}{Tab}{/Shift}'); expect(screen.getByRole('button', { name: 'Delete permanently' })).toHaveFocus(); });
  it('closes once on Escape', async () => { const user = userEvent.setup(); const close = vi.fn(); render(<Dialog title="x" onClose={close}><button>Cancel</button></Dialog>); await user.keyboard('{Escape}'); expect(close).toHaveBeenCalledTimes(1); });
  it('restores trigger focus', () => { const trigger = document.body.appendChild(document.createElement('button')); trigger.textContent = 'Open'; trigger.focus(); const { unmount } = render(modal()); unmount(); expect(trigger).toHaveFocus(); });
  it('excludes disabled controls', () => { render(modal(<><button disabled>Disabled</button><button>Enabled</button></>)); expect(screen.getByRole('button', { name: 'Enabled' })).toHaveFocus(); });
  it('focuses itself without controls and works without root', async () => { const user = userEvent.setup(); render(modal(<p>Only text</p>)); expect(screen.getByRole('dialog')).toHaveFocus(); await user.keyboard('{Escape}'); });
});
