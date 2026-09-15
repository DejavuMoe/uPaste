import React, { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

export type DeploymentMode = 'private' | 'public';
export type ChallengeProvider = 'cap' | 'turnstile';

export interface RetentionPolicy {
  default_seconds: number;
  max_seconds: number;
}

export interface PublicChallengeConfig {
  provider: ChallengeProvider;
  site_key: string;
  api_endpoint?: string;
  hostname?: string;
}

export interface DeploymentConfig {
  deployment_mode: DeploymentMode;
  admin_enabled: boolean;
  retention?: RetentionPolicy;
  challenge?: PublicChallengeConfig;
}

const PRIVATE_DEFAULT: DeploymentConfig = { deployment_mode: 'private', admin_enabled: false };

interface ConfigContextValue {
  config: DeploymentConfig;
  loading: boolean;
  error: boolean;
  reload: () => void;
}

const ConfigContext = createContext<ConfigContextValue>({
  config: PRIVATE_DEFAULT,
  loading: false,
  error: false,
  reload: () => {},
});

export const ConfigProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [config, setConfig] = useState<DeploymentConfig>(PRIVATE_DEFAULT);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<boolean>(false);
  const [reloadKey, setReloadKey] = useState<number>(0);

  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    setLoading(true);
    setError(false);
    fetch('/api/v1/config', { method: 'GET', signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) throw new Error('config unavailable');
        return (await response.json()) as DeploymentConfig;
      })
      .then((value) => {
        if (!active) return;
        setConfig(value);
      })
      .catch((err) => {
        if (!active || err?.name === 'AbortError') return;
        // Fail closed for public behavior: keep private defaults only until the
        // operator's policy can be loaded, and surface a loading error.
        setError(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      controller.abort();
    };
  }, [reloadKey]);

  const value = useMemo(
    () => ({ config, loading, error, reload: () => setReloadKey((key) => key + 1) }),
    [config, loading, error],
  );
  return <ConfigContext.Provider value={value}>{children}</ConfigContext.Provider>;
};

export function useDeploymentConfig(): ConfigContextValue {
  return useContext(ConfigContext);
}

export function isPublicConfig(config: DeploymentConfig): boolean {
  return config.deployment_mode === 'public';
}
