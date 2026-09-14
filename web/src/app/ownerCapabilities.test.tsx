import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import React from 'react';
import { OwnerCapabilityProvider, useOwnerCapabilities } from './ownerCapabilities';

describe('OwnerCapabilities', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  const wrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
    <OwnerCapabilityProvider>{children}</OwnerCapabilityProvider>
  );

  it('stores and retrieves capabilities in volatile memory', () => {
    const { result } = renderHook(() => useOwnerCapabilities(), { wrapper });

    expect(result.current.get('share-1')).toBeUndefined();

    act(() => {
      result.current.remember('share-1', 'up_o1_token123');
    });

    expect(result.current.get('share-1')).toBe('up_o1_token123');

    // Verify it is NOT leaked to localStorage or sessionStorage
    expect(localStorage.getItem('share-1')).toBeNull();
    expect(localStorage.getItem('up_o1_token123')).toBeNull();
    expect(sessionStorage.getItem('share-1')).toBeNull();
    expect(sessionStorage.getItem('up_o1_token123')).toBeNull();
    expect(window.history.state).toBeNull();
  });

  it('removes capabilities with forget and clear', () => {
    const { result } = renderHook(() => useOwnerCapabilities(), { wrapper });

    act(() => {
      result.current.remember('share-1', 'token-1');
      result.current.remember('share-2', 'token-2');
    });

    expect(result.current.get('share-1')).toBe('token-1');
    expect(result.current.get('share-2')).toBe('token-2');

    act(() => {
      result.current.forget('share-1');
    });

    expect(result.current.get('share-1')).toBeUndefined();
    expect(result.current.get('share-2')).toBe('token-2');

    act(() => {
      result.current.clear();
    });

    expect(result.current.get('share-2')).toBeUndefined();
  });

  it('throws when useOwnerCapabilities is used outside provider', () => {
    expect(() => renderHook(() => useOwnerCapabilities())).toThrow(
      /useOwnerCapabilities must be used within an OwnerCapabilityProvider/,
    );
  });
});
