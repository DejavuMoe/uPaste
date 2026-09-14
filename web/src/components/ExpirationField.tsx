import React, { useState, useEffect, useId } from 'react';

export type ExpirationPreset = 'never' | '1h' | '1d' | '7d' | '30d' | 'custom';

export interface ExpirationValue {
  preset: ExpirationPreset;
  customIso?: string; // local ISO string YYYY-MM-DDTHH:mm
  error?: string;
  // Function to calculate RFC3339 UTC timestamp at submission time
  resolveExpiresAt: () => string | null;
}

export interface ExpirationFieldProps {
  id?: string;
  value?: ExpirationPreset;
  onChange?: (val: ExpirationValue) => void;
  disabled?: boolean;
  className?: string;
}

export function computeExpiresAt(preset: ExpirationPreset, customLocalValue?: string): string | null {
  const now = Date.now();
  switch (preset) {
    case 'never':
      return null;
    case '1h':
      return new Date(now + 60 * 60 * 1000).toISOString();
    case '1d':
      return new Date(now + 24 * 60 * 60 * 1000).toISOString();
    case '7d':
      return new Date(now + 7 * 24 * 60 * 60 * 1000).toISOString();
    case '30d':
      return new Date(now + 30 * 24 * 60 * 60 * 1000).toISOString();
    case 'custom': {
      if (!customLocalValue) return null;
      const parsed = new Date(customLocalValue);
      if (isNaN(parsed.getTime())) return null;
      return parsed.toISOString();
    }
  }
}

export const ExpirationField: React.FC<ExpirationFieldProps> = ({
  id,
  value = 'never',
  onChange,
  disabled = false,
  className = '',
}) => {
  const generatedId = useId();
  const selectId = id || `exp-select-${generatedId}`;
  const customInputId = `exp-custom-${generatedId}`;

  const [preset, setPreset] = useState<ExpirationPreset>(value);
  const [customValue, setCustomValue] = useState<string>('');
  const [error, setError] = useState<string | undefined>(undefined);

  useEffect(() => {
    let currentError: string | undefined = undefined;

    if (preset === 'custom') {
      if (!customValue) {
        currentError = 'Please select a custom expiration date.';
      } else {
        const parsed = new Date(customValue);
        if (isNaN(parsed.getTime())) {
          currentError = 'Invalid date format.';
        } else if (parsed.getTime() <= Date.now()) {
          currentError = 'Expiration date must be in the future.';
        }
      }
    }

    setError(currentError);

    if (onChange) {
      onChange({
        preset,
        customIso: customValue,
        error: currentError,
        resolveExpiresAt: () => {
          if (currentError) return null;
          return computeExpiresAt(preset, customValue);
        },
      });
    }
  }, [preset, customValue, onChange]);

  return (
    <div className={`expiration-field ${className}`.trim()}>
      <div className="expiration-controls">
        <label htmlFor={selectId} className="sr-only">
          Expiration
        </label>
        <select
          id={selectId}
          className="form-select"
          value={preset}
          disabled={disabled}
          onChange={(e) => setPreset(e.target.value as ExpirationPreset)}
          aria-label="Expiration"
        >
          <option value="never">Never</option>
          <option value="1h">1 hour</option>
          <option value="1d">1 day</option>
          <option value="7d">7 days</option>
          <option value="30d">30 days</option>
          <option value="custom">Custom…</option>
        </select>

        {preset === 'custom' && (
          <div className="custom-datetime-container">
            <label htmlFor={customInputId} className="sr-only">
              Custom expiration date and time
            </label>
            <input
              id={customInputId}
              type="datetime-local"
              className={`form-input custom-datetime-input ${error ? 'input-error' : ''}`}
              value={customValue}
              disabled={disabled}
              onChange={(e) => setCustomValue(e.target.value)}
              aria-invalid={!!error}
              aria-describedby={error ? `${customInputId}-error` : undefined}
            />
          </div>
        )}
      </div>
      {preset === 'custom' && error && (
        <div id={`${customInputId}-error`} className="form-error" role="alert">
          {error}
        </div>
      )}
    </div>
  );
};
