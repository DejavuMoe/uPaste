import React, { useState, useEffect, useId, useMemo } from 'react';

export type ExpirationPreset = 'never' | '1h' | '1d' | '7d' | '30d' | 'custom';

export interface ExpirationValue {
  preset: ExpirationPreset;
  customIso?: string;
  error?: string;
  resolveExpiresAt: () => string | null;
}

export interface ExpirationPolicy {
  mode: 'private' | 'public';
  defaultSeconds?: number;
  maxSeconds?: number;
}

export interface ExpirationFieldProps {
  id?: string;
  value?: ExpirationPreset;
  onChange?: (val: ExpirationValue) => void;
  disabled?: boolean;
  className?: string;
  policy?: ExpirationPolicy;
}

const PRESETS: Array<{ preset: ExpirationPreset; label: string; seconds?: number }> = [
  { preset: 'never', label: 'Never' },
  { preset: '1h', label: '1 hour', seconds: 60 * 60 },
  { preset: '1d', label: '1 day', seconds: 24 * 60 * 60 },
  { preset: '7d', label: '7 days', seconds: 7 * 24 * 60 * 60 },
  { preset: '30d', label: '30 days', seconds: 30 * 24 * 60 * 60 },
  { preset: 'custom', label: 'Custom…' },
];

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

function localDateTimeValue(date: Date): string {
  const shifted = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return shifted.toISOString().slice(0, 16);
}

function presetForSeconds(seconds: number): ExpirationPreset | null {
  const found = PRESETS.find((option) => option.seconds === seconds);
  return found && found.preset !== 'custom' ? found.preset : null;
}

export const ExpirationField: React.FC<ExpirationFieldProps> = ({
  id,
  value = 'never',
  onChange,
  disabled = false,
  className = '',
  policy,
}) => {
  const generatedId = useId();
  const selectId = id || `exp-select-${generatedId}`;
  const customInputId = `exp-custom-${generatedId}`;

  const isPublic = policy?.mode === 'public';
  const defaultSeconds = policy?.defaultSeconds;
  const maxSeconds = policy?.maxSeconds;

  const initialPreset = useMemo<ExpirationPreset>(() => {
    if (!isPublic) return value;
    if (value !== 'never' && value !== '30d') return value;
    if (defaultSeconds) {
      return presetForSeconds(defaultSeconds) ?? 'custom';
    }
    return '1d';
  }, [isPublic, value, defaultSeconds]);

  const [preset, setPreset] = useState<ExpirationPreset>(initialPreset);
  const [customValue, setCustomValue] = useState<string>(() => {
    if (isPublic && initialPreset === 'custom' && defaultSeconds) {
      return localDateTimeValue(new Date(Date.now() + defaultSeconds * 1000));
    }
    return '';
  });
  const [error, setError] = useState<string | undefined>(undefined);

  const options = useMemo(() => {
    if (!isPublic) return PRESETS;
    return PRESETS.filter((option) => {
      if (option.preset === 'never' || option.preset === '30d') return false;
      if (option.seconds === undefined) return true; // custom
      return maxSeconds === undefined || option.seconds <= maxSeconds;
    });
  }, [isPublic, maxSeconds]);

  // Configuration loads asynchronously; when public policy becomes available,
  // replace a private-only default (for example "never") with the public default.
  useEffect(() => {
    if (!isPublic) return;
    setPreset((current) => (options.some((option) => option.preset === current) && current !== 'never' ? current : initialPreset));
    if (initialPreset === 'custom' && defaultSeconds) {
      setCustomValue((current) => current || localDateTimeValue(new Date(Date.now() + defaultSeconds * 1000)));
    }
  }, [isPublic, initialPreset, options, defaultSeconds]);

  useEffect(() => {
    let currentError: string | undefined;
    if (isPublic && preset === 'never') {
      currentError = 'Public Shares always expire.';
    } else if (preset === 'custom') {
      if (!customValue) {
        currentError = 'Please select a custom expiration date.';
      } else {
        const parsed = new Date(customValue);
        if (isNaN(parsed.getTime())) {
          currentError = 'Invalid date format.';
        } else if (parsed.getTime() <= Date.now()) {
          currentError = 'Expiration date must be in the future.';
        } else if (isPublic && maxSeconds !== undefined && parsed.getTime() > Date.now() + maxSeconds * 1000) {
          currentError = `Expiration must be within ${Math.floor(maxSeconds / 3600)} hours of now.`;
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
  }, [preset, customValue, onChange, isPublic, maxSeconds]);

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
          onChange={(event) => setPreset(event.target.value as ExpirationPreset)}
          aria-label="Expiration"
        >
          {options.map((option) => (
            <option key={option.preset} value={option.preset}>
              {option.label}
            </option>
          ))}
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
              onChange={(event) => setCustomValue(event.target.value)}
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
