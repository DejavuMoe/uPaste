import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import React from 'react';

const capInstances: any[] = [];
vi.mock('@cap.js/widget', () => {
  return {
    default: class MockCap {
      listeners: Record<string, (event: any) => void> = {};
      reset = vi.fn();
      removeEventListener = vi.fn();
      solve = vi.fn(async () => {});
      constructor() {
        capInstances.push(this);
      }
      addEventListener(type: string, listener: (event: any) => void) {
        this.listeners[type] = listener;
      }
      emit(type: string, detail: any) {
        this.listeners[type]?.({ detail });
      }
    },
  };
});
vi.mock('@cap.js/wasm/browser/cap_wasm_bg.wasm?url', () => ({ default: '/assets/cap-wasm.wasm' }));

import { ChallengeGate } from './ChallengeGate';

describe('ChallengeGate', () => {
  beforeEach(() => {
    capInstances.length = 0;
    delete (window as any).turnstile;
  });

  it('propagates a Turnstile token and removes the widget', async () => {
    const remove = vi.fn();
    (window as any).turnstile = {
      render: vi.fn((_element: HTMLElement, options: any) => {
        options.callback('turnstile-token');
        return 'widget-1';
      }),
      remove,
      reset: vi.fn(),
    };
    const onToken = vi.fn();
    const { unmount } = render(
      <ChallengeGate challenge={{ provider: 'turnstile', site_key: 'site-key' }} onToken={onToken} />,
    );
    await waitFor(() => expect(onToken).toHaveBeenCalledWith('turnstile-token'));
    expect(screen.getByText('Verified')).toBeInTheDocument();
    unmount();
    expect(remove).toHaveBeenCalledWith('widget-1');
  });

  it('propagates a Cap solve token from the pinned widget wrapper', async () => {
    const onToken = vi.fn();
    render(
      <ChallengeGate
        challenge={{ provider: 'cap', site_key: 'cap-site', api_endpoint: 'https://cap.example.com' }}
        onToken={onToken}
      />,
    );
    await waitFor(() => expect(capInstances.length).toBe(1));
    capInstances[0].emit('solve', { token: 'cap-token' });
    await waitFor(() => expect(onToken).toHaveBeenCalledWith('cap-token'));
    expect(screen.getByText('Verified')).toBeInTheDocument();
    expect((window as any).CAP_CUSTOM_WASM_URL).toBe('/assets/cap-wasm.wasm');
  });
});
