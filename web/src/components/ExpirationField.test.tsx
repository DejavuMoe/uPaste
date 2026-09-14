import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { ExpirationField, computeExpiresAt } from './ExpirationField';

describe('ExpirationField', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('computeExpiresAt', () => {
    it('computes null for never', () => {
      expect(computeExpiresAt('never')).toBeNull();
    });

    it('computes future timestamps for presets', () => {
      const now = 1700000000000;
      vi.spyOn(Date, 'now').mockReturnValue(now);

      const exp1h = computeExpiresAt('1h');
      expect(exp1h).toBe(new Date(now + 3600000).toISOString());

      const exp1d = computeExpiresAt('1d');
      expect(exp1d).toBe(new Date(now + 86400000).toISOString());

      const exp7d = computeExpiresAt('7d');
      expect(exp7d).toBe(new Date(now + 7 * 86400000).toISOString());

      const exp30d = computeExpiresAt('30d');
      expect(exp30d).toBe(new Date(now + 30 * 86400000).toISOString());
    });

    it('parses valid custom datetime', () => {
      const customInput = '2026-10-15T14:30';
      const expectedIso = new Date('2026-10-15T14:30').toISOString();
      expect(computeExpiresAt('custom', customInput)).toBe(expectedIso);
    });

    it('returns null for empty or invalid custom datetime', () => {
      expect(computeExpiresAt('custom', '')).toBeNull();
      expect(computeExpiresAt('custom', 'invalid-date')).toBeNull();
    });
  });

  describe('ExpirationField component', () => {
    it('renders presets dropdown with Never as default', () => {
      render(<ExpirationField />);
      const select = screen.getByRole('combobox', { name: /expiration/i });
      expect(select).toBeInTheDocument();
      expect((select as HTMLSelectElement).value).toBe('never');
    });

    it('shows custom datetime input when Custom… is selected', () => {
      const handleChange = vi.fn();
      render(<ExpirationField onChange={handleChange} />);

      const select = screen.getByRole('combobox', { name: /expiration/i });
      fireEvent.change(select, { target: { value: 'custom' } });

      const customInput = screen.getByLabelText(/custom expiration date and time/i);
      expect(customInput).toBeInTheDocument();
      expect(screen.getByRole('alert')).toHaveTextContent(/select a custom expiration date/i);
    });

    it('validates custom date must be in the future', () => {
      const now = 1700000000000;
      vi.spyOn(Date, 'now').mockReturnValue(now);

      render(<ExpirationField />);
      const select = screen.getByRole('combobox', { name: /expiration/i });
      fireEvent.change(select, { target: { value: 'custom' } });

      const customInput = screen.getByLabelText(/custom expiration date and time/i);

      // Set past date
      fireEvent.change(customInput, { target: { value: '2020-01-01T00:00' } });
      expect(screen.getByRole('alert')).toHaveTextContent(/expiration date must be in the future/i);

      // Set future date (2 days ahead)
      const target = new Date(now + 2 * 86400000);
      const pad = (n: number) => String(n).padStart(2, '0');
      const futureDate = `${target.getFullYear()}-${pad(target.getMonth() + 1)}-${pad(target.getDate())}T${pad(target.getHours())}:${pad(target.getMinutes())}`;
      fireEvent.change(customInput, { target: { value: futureDate } });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });
});
